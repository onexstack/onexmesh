// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ratelimit

import (
	"context"
	"fmt"
	"time"
)

// PeriodLimit result codes. Unlike a boolean limiter, these let callers
// distinguish "allowed", "allowed but just hit the quota", and "rejected".
const (
	// Unknown indicates the outcome could not be determined (store error).
	Unknown = iota
	// Allowed means the request is within quota and quota is not yet full.
	Allowed
	// HitQuota means the request is within quota and just filled it.
	HitQuota
	// OverQuota means the request exceeds the quota.
	OverQuota
)

// PeriodOption customizes a PeriodLimit.
type PeriodOption func(*PeriodLimit)

// Align aligns the window to natural boundaries (e.g. a 1-day period resets at
// midnight rather than one day after first use).
func Align() PeriodOption {
	return func(l *PeriodLimit) { l.align = true }
}

// PeriodLimit limits how many requests are allowed within a rolling (or aligned)
// time window, backed by a shared Store for cross-instance coordination.
type PeriodLimit struct {
	period    time.Duration
	quota     int
	store     Store
	keyPrefix string
	align     bool
}

// NewPeriodLimit returns a PeriodLimit allowing quota requests per period,
// keyed under keyPrefix.
func NewPeriodLimit(period time.Duration, quota int, store Store, keyPrefix string,
	opts ...PeriodOption) *PeriodLimit {
	l := &PeriodLimit{period: period, quota: quota, store: store, keyPrefix: keyPrefix}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Take records one request for key and reports its result code.
func (l *PeriodLimit) Take(key string) (int, error) {
	return l.TakeCtx(context.Background(), key)
}

// TakeCtx is Take with a context.
func (l *PeriodLimit) TakeCtx(ctx context.Context, key string) (int, error) {
	k := l.keyPrefix + key
	if l.align {
		sec := int64(l.period.Seconds())
		if sec < 1 {
			sec = 1
		}
		k = fmt.Sprintf("%s:%d", k, time.Now().Unix()/sec)
	}

	count, err := l.store.IncrWindow(ctx, k, l.period)
	if err != nil {
		return Unknown, err
	}

	switch {
	case count > int64(l.quota):
		return OverQuota, nil
	case count == int64(l.quota):
		return HitQuota, nil
	default:
		return Allowed, nil
	}
}
