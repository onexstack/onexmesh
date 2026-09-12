// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package executors provides batching executors that coalesce sporadic tasks
// into fewer, larger executions: periodical (time-driven), bulk (count-driven),
// delay (debounced), chunk (byte-size-driven) and less (rate-limited).
package executors

import (
	"log/slog"
	"time"
)

// defaultFlushInterval is the default flush interval shared by executors.
const defaultFlushInterval = time.Second

// Execute handles a batch of tasks.
type Execute func(tasks []any)

// safeGo runs fn in a fresh goroutine, recovering and logging any panic so a
// single misbehaving task cannot crash the process.
func safeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("executor task panicked", "panic", r)
			}
		}()
		fn()
	}()
}
