// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// HedgeOption configures the Hedge middleware.
type HedgeOption func(*hedgeConfig)

type hedgeConfig struct {
	delay         time.Duration
	maxBackups    int
	backupErrRate float64
}

// WithBackupMaxRetries sets how many backup requests to issue once the primary
// exceeds delay. It is clamped to [1, 2]; default 1.
func WithBackupMaxRetries(n int) HedgeOption {
	return func(c *hedgeConfig) {
		if n < 1 {
			n = 1
		}
		if n > 2 {
			n = 2
		}
		c.maxBackups = n
	}
}

// WithBackupErrorRate enables adaptive throttling of backups: when the observed
// backup error rate reaches rate, no backup is issued (the primary is awaited
// instead), preventing extra load from being piled onto an already-failing
// downstream. rate must be in (0, 1]; a value of 0 disables the throttle.
func WithBackupErrorRate(rate float64) HedgeOption {
	return func(c *hedgeConfig) {
		if rate < 0 {
			rate = 0
		}
		if rate > 1 {
			rate = 1
		}
		c.backupErrRate = rate
	}
}

// Hedge returns a middleware that issues backup (hedged) requests when the
// primary request does not complete within delay, returning whichever finishes
// first. It trades a bounded amount of extra load for lower tail latency.
//
// Because it runs the handler concurrently, the handler must be safe to execute
// in parallel (idempotent and side-effect-free with respect to shared state).
// It is therefore suited to idempotent calls such as HTTP GET; do not wrap a
// handler that mutates a shared request/response object.
func Hedge(delay time.Duration, opts ...HedgeOption) Middleware {
	cfg := &hedgeConfig{delay: delay, maxBackups: 1}
	for _, o := range opts {
		o(cfg)
	}

	var backupSuccess, backupTotal atomic.Int64

	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			// A derived context lets us cancel the losing requests so they do not
			// keep running (and leaking) after the winner has returned.
			hctx, cancel := context.WithCancel(ctx)
			defer cancel()

			primary := make(chan error, 1)
			go runHedge(primary, hctx, next)

			timer := time.NewTimer(cfg.delay)
			defer timer.Stop()

			select {
			case err := <-primary:
				return err
			case <-timer.C:
			}

			// Adaptive throttle: back off backups when they are mostly failing.
			if cfg.backupErrRate > 0 {
				if total := backupTotal.Load(); total > 0 {
					errRate := float64(total-backupSuccess.Load()) / float64(total)
					if errRate >= cfg.backupErrRate {
						return <-primary
					}
				}
			}

			backup := make(chan error, cfg.maxBackups)
			for i := 0; i < cfg.maxBackups; i++ {
				go runHedge(backup, hctx, next)
			}

			select {
			case err := <-primary:
				return err
			case err := <-backup:
				backupTotal.Add(1)
				if err == nil {
					backupSuccess.Add(1)
				}
				return err
			}
		}
	}
}

// runHedge executes next and sends its result to out. It recovers panics so a
// handler panic does not crash the process; the panic is surfaced as an error
// instead, consistent with how a middleware panic is normally handled.
func runHedge(out chan<- error, ctx context.Context, next Handler) {
	defer func() {
		if r := recover(); r != nil {
			out <- fmt.Errorf("resilience: hedge panic: %v", r)
		}
	}()
	out <- next(ctx)
}
