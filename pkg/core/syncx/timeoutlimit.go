// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import (
	"errors"
	"time"

	"github.com/onexstack/onexmesh/pkg/core/limit"
)

// ErrTimeout indicates a borrow timed out.
var ErrTimeout = errors.New("syncx: borrow timeout")

// TimeoutLimit bounds concurrent borrows with a timeout, built on the channel
// semaphore from pkg/core/limit plus a Cond to wake blocked borrowers.
type TimeoutLimit struct {
	limit limit.Limit
	cond  *Cond
}

// NewTimeoutLimit returns a TimeoutLimit of n concurrent borrows.
func NewTimeoutLimit(n int) TimeoutLimit {
	return TimeoutLimit{limit: limit.NewLimit(n), cond: NewCond()}
}

// Borrow borrows with a timeout, returning ErrTimeout when it cannot acquire a
// slot in time.
func (l TimeoutLimit) Borrow(timeout time.Duration) error {
	if l.TryBorrow() {
		return nil
	}

	var ok bool
	for {
		timeout, ok = l.cond.WaitWithTimeout(timeout)
		if ok && l.TryBorrow() {
			return nil
		}
		if timeout <= 0 {
			return ErrTimeout
		}
	}
}

// Return returns a borrowed slot, waking one blocked borrower.
func (l TimeoutLimit) Return() error {
	if err := l.limit.Return(); err != nil {
		return err
	}
	l.cond.Signal()
	return nil
}

// TryBorrow borrows without blocking.
func (l TimeoutLimit) TryBorrow() bool {
	return l.limit.TryBorrow()
}
