// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import "time"

// Cond is a channel-based condition primitive that supports timed waits. It is
// used by TimeoutLimit to wake borrowers when a resource is returned.
type Cond struct {
	signal chan struct{}
}

// NewCond returns a Cond.
func NewCond() *Cond {
	return &Cond{signal: make(chan struct{})}
}

// WaitWithTimeout blocks for a signal, returning the remaining timeout and
// whether a signal (rather than the timeout) was received.
func (c *Cond) WaitWithTimeout(timeout time.Duration) (time.Duration, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	begin := time.Now()
	select {
	case <-c.signal:
		elapsed := time.Since(begin)
		return timeout - elapsed, true
	case <-timer.C:
		return 0, false
	}
}

// Wait blocks until a signal.
func (c *Cond) Wait() {
	<-c.signal
}

// Signal wakes one waiting goroutine, if any.
func (c *Cond) Signal() {
	select {
	case c.signal <- struct{}{}:
	default:
	}
}
