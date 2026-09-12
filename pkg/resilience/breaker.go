// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import (
	"context"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/onexstack/onexmesh/pkg/core/collection"
	"github.com/onexstack/onexmesh/pkg/errno"
)

// ErrCircuitOpen is returned when the breaker is open and rejects the request.
// It aliases errno.ErrCircuitOpen so callers can match it with errors.Is while
// the sentinel stays centrally defined.
var ErrCircuitOpen = errno.ErrCircuitOpen

// SRE breaker tuning constants, following the Google "Handling Overload"
// client-side throttling guidance (see SRE book, Client-Side Throttling).
const (
	minK          = 1.1 // minimum accept multiplier when heavily failing
	protection    = 5   // minimum requests to allow before throttling kicks in
	defaultWindow = 10 * time.Second
	defaultBucket = 40
	defaultK      = 1.5
	defaultProbe  = time.Second
)

// BreakerOption configures the circuit breaker.
type BreakerOption func(*breakerConfig)

type breakerConfig struct {
	window        time.Duration
	buckets       int
	k             float64
	probeInterval time.Duration
	acceptable    func(error) bool
}

func defaultBreakerConfig() *breakerConfig {
	return &breakerConfig{
		window:        defaultWindow,
		buckets:       defaultBucket,
		k:             defaultK,
		probeInterval: defaultProbe,
		acceptable:    func(err error) bool { return err == nil },
	}
}

// WithWindow sets the sliding window duration. Default 10s.
func WithWindow(d time.Duration) BreakerOption {
	return func(c *breakerConfig) { c.window = d }
}

// WithBuckets sets the number of buckets in the window. Default 40.
func WithBuckets(n int) BreakerOption {
	return func(c *breakerConfig) { c.buckets = n }
}

// WithK sets the accept multiplier k. Default 1.5.
func WithK(k float64) BreakerOption {
	return func(c *breakerConfig) { c.k = k }
}

// WithProbeInterval sets how often a request is force-passed while throttling,
// so the breaker can recover once the downstream heals. Default 1s.
func WithProbeInterval(d time.Duration) BreakerOption {
	return func(c *breakerConfig) { c.probeInterval = d }
}

// WithAcceptable sets the predicate that classifies an error as an acceptable
// outcome (not a failure). Errors judged acceptable do not trip the breaker;
// this lets callers treat e.g. timeouts or 4xx client errors as non-failures.
// Default treats any non-nil error as a failure.
func WithAcceptable(f func(error) bool) BreakerOption {
	return func(c *breakerConfig) { c.acceptable = f }
}

// Breaker returns a middleware guarding the handler with a Google SRE
// sliding-window circuit breaker. It probabilistically rejects requests as the
// failure rate rises instead of using a hard open/close state machine, so it
// degrades gracefully rather than flapping.
func Breaker(opts ...BreakerOption) Middleware {
	cfg := defaultBreakerConfig()
	for _, o := range opts {
		o(cfg)
	}
	b := newSREBreaker(cfg)
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			return b.doReq(func() error { return next(ctx) }, cfg.acceptable)
		}
	}
}

// NopBreaker returns a middleware that never trips the circuit.
func NopBreaker() Middleware {
	return func(next Handler) Handler { return next }
}

// sreBreaker implements the Google SRE client-side throttling algorithm.
type sreBreaker struct {
	k             float64
	buckets       int
	stat          *collection.RollingWindow[int64, *breakerBucket]
	probeInterval time.Duration
	lastPass      atomic.Int64
}

// result codes recorded in each bucket.
const (
	resultSuccess = iota
	resultFailure
	resultDrop
)

// breakerBucket aggregates outcomes over a single time bucket.
type breakerBucket struct {
	Sum     int64
	Success int64
	Failure int64
	Drop    int64
}

func (b *breakerBucket) Add(v int64) {
	switch v {
	case resultFailure:
		b.Sum++
		b.Failure++
	case resultDrop:
		b.Sum++
		b.Drop++
	default:
		b.Sum++
		b.Success++
	}
}

func (b *breakerBucket) Reset() {
	b.Sum = 0
	b.Success = 0
	b.Failure = 0
	b.Drop = 0
}

type windowResult struct {
	accepts        int64
	total          int64
	failingBuckets int64
	workingBuckets int64
}

func newSREBreaker(cfg *breakerConfig) *sreBreaker {
	buckets := cfg.buckets
	if buckets < 1 {
		buckets = defaultBucket
	}
	bucketDuration := cfg.window / time.Duration(buckets)
	stat := collection.NewRollingWindow(func() *breakerBucket {
		return &breakerBucket{}
	}, buckets, bucketDuration)

	return &sreBreaker{
		k:             cfg.k,
		buckets:       buckets,
		stat:          stat,
		probeInterval: cfg.probeInterval,
	}
}

// accept decides whether to allow the request, returning ErrCircuitOpen to drop.
func (b *sreBreaker) accept() error {
	history := b.history()
	w := b.k - (b.k-minK)*float64(history.failingBuckets)/float64(b.buckets)
	weightedAccepts := atLeast(w, minK) * float64(history.accepts)

	// SRE formula: dropRatio = (total - protection - k*accepts) / (total + 1).
	dropRatio := (float64(history.total-protection) - weightedAccepts) / float64(history.total+1)
	if dropRatio <= 0 {
		return nil
	}

	// Force-pass one request after the probe interval so the breaker can recover
	// once the downstream heals.
	lastPass := b.lastPass.Load()
	if lastPass > 0 && time.Since(time.Unix(0, lastPass)) > b.probeInterval {
		b.lastPass.Store(time.Now().UnixNano())
		return nil
	}

	// Reduce the drop ratio while the window is still warming up.
	dropRatio *= float64(int64(b.buckets)-history.workingBuckets) / float64(b.buckets)

	if rand.Float64() < dropRatio {
		return ErrCircuitOpen
	}
	b.lastPass.Store(time.Now().UnixNano())
	return nil
}

func (b *sreBreaker) doReq(req func() error, acceptable func(error) bool) error {
	if err := b.accept(); err != nil {
		b.markDrop()
		return err
	}

	var succ bool
	defer func() {
		if succ {
			b.markSuccess()
		} else {
			b.markFailure()
		}
	}()

	err := req()
	if acceptable(err) {
		succ = true
	}
	return err
}

func (b *sreBreaker) markDrop()    { b.stat.Add(resultDrop) }
func (b *sreBreaker) markFailure() { b.stat.Add(resultFailure) }
func (b *sreBreaker) markSuccess() { b.stat.Add(resultSuccess) }

// healthy reports whether the breaker would currently allow a request, ignoring
// the force-pass probe. It is used by the selector to skip breaker-tripped
// nodes without mutating breaker state.
func (b *sreBreaker) healthy() bool {
	h := b.history()
	w := b.k - (b.k-minK)*float64(h.failingBuckets)/float64(b.buckets)
	weightedAccepts := atLeast(w, minK) * float64(h.accepts)
	dropRatio := (float64(h.total-protection) - weightedAccepts) / float64(h.total+1)
	return dropRatio <= 0
}

func (b *sreBreaker) history() windowResult {
	var result windowResult
	b.stat.Reduce(func(b *breakerBucket) {
		result.accepts += b.Success
		result.total += b.Sum
		if b.Success > 0 {
			result.workingBuckets++
		}
		if b.Failure > 0 {
			result.failingBuckets++
		}
	})
	return result
}

// atLeast returns the greater of x and lower.
func atLeast(x, lower float64) float64 {
	if x < lower {
		return lower
	}
	return x
}
