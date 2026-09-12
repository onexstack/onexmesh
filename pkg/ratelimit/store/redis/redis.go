// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package redis provides a Redis-backed ratelimit.Store using atomic Lua
// scripts, for distributed rate limiting across multiple instances.
package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/onexstack/onexmesh/pkg/ratelimit"
)

var _ ratelimit.Store = (*Store)(nil)

// tokenScript atomically refills and consumes a token bucket. KEYS[1] holds the
// current token count, KEYS[2] the last-refill timestamp (unix ms). ARGV are
// rate (tokens/sec), burst, now (unix ms), requested tokens and the key TTL
// (ms). Both keys are given a TTL so an idle limiter does not leak keys.
const tokenScript = `
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

local tokens = tonumber(redis.call("GET", KEYS[1]))
if tokens == nil then tokens = burst end
local last = tonumber(redis.call("GET", KEYS[2]))
if last == nil then last = now end

local filled = math.min(burst, tokens + (math.max(0, now - last) / 1000) * rate)
if filled >= requested then
    redis.call("SET", KEYS[1], filled - requested)
    redis.call("SET", KEYS[2], now)
    redis.call("PEXPIRE", KEYS[1], ttl)
    redis.call("PEXPIRE", KEYS[2], ttl)
    return 1
end
redis.call("SET", KEYS[1], filled)
redis.call("SET", KEYS[2], now)
redis.call("PEXPIRE", KEYS[1], ttl)
redis.call("PEXPIRE", KEYS[2], ttl)
return 0
`

// windowScript atomically increments a counter and sets expiry on first use.
// ARGV[1] is the window duration in milliseconds, so sub-second windows expire
// correctly instead of being deleted immediately by an EXPIRE of 0.
const windowScript = `
local current = redis.call("INCRBY", KEYS[1], 1)
if current == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`

// Store is a Redis-backed Store.
type Store struct {
	client       redis.UniversalClient
	tokenScript  *redis.Script
	windowScript *redis.Script
}

// New returns a Store backed by client.
func New(client redis.UniversalClient) *Store {
	return &Store{
		client:       client,
		tokenScript:  redis.NewScript(tokenScript),
		windowScript: redis.NewScript(windowScript),
	}
}

// NewFromAddr returns a Store backed by a client connecting to addr.
func NewFromAddr(addr string) *Store {
	return New(redis.NewClient(&redis.Options{Addr: addr}))
}

// TakeTokens implements ratelimit.Store.
func (s *Store) TakeTokens(ctx context.Context, key string, rate, burst, n int) (bool, error) {
	keys := []string{key + ":tokens", key + ":ts"}
	now := time.Now().UnixMilli()
	res, err := s.tokenScript.Run(ctx, s.client, keys, rate, burst, now, n, tokenTTLMillis(rate, burst)).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// IncrWindow implements ratelimit.Store.
func (s *Store) IncrWindow(ctx context.Context, key string, window time.Duration) (int64, error) {
	ttl := window.Milliseconds()
	if ttl < 1 {
		ttl = 1
	}
	return s.windowScript.Run(ctx, s.client, []string{key}, ttl).Int64()
}

// tokenTTLMillis returns a key TTL in milliseconds for a token bucket. It is
// twice the time needed to refill the bucket from empty, bounded to at least
// one second, so idle limiter keys eventually expire instead of leaking.
func tokenTTLMillis(rate, burst int) int64 {
	if rate <= 0 {
		return int64(time.Minute / time.Millisecond)
	}
	fill := int64(burst) * int64(time.Second/time.Millisecond) / int64(rate)
	if fill < int64(time.Second/time.Millisecond) {
		fill = int64(time.Second / time.Millisecond)
	}
	return 2 * fill
}

// Ping implements ratelimit.Store.
func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Close implements ratelimit.Store.
func (s *Store) Close() error {
	return s.client.Close()
}
