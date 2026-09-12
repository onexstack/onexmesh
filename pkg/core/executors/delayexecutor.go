// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package executors

import (
	"sync"
	"time"
)

// DelayExecutor runs a task after a debounce delay, coalescing repeated
// triggers within the delay window.
type DelayExecutor struct {
	fn        func()
	delay     time.Duration
	triggered bool
	lock      sync.Mutex
}

// NewDelayExecutor returns a DelayExecutor running fn after delay.
func NewDelayExecutor(fn func(), delay time.Duration) *DelayExecutor {
	return &DelayExecutor{fn: fn, delay: delay}
}

// Trigger schedules fn to run after the delay. It is safe to call repeatedly;
// subsequent triggers within the delay window are coalesced.
func (de *DelayExecutor) Trigger() {
	de.lock.Lock()
	defer de.lock.Unlock()

	if de.triggered {
		return
	}
	de.triggered = true

	safeGo(func() {
		timer := time.NewTimer(de.delay)
		defer timer.Stop()
		<-timer.C

		// Clear triggered before running fn so no trigger is missed.
		de.lock.Lock()
		de.triggered = false
		de.lock.Unlock()
		de.fn()
	})
}
