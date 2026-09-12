// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package limit provides a channel-based semaphore for bounding concurrency.
package limit

import "errors"

// ErrLimitReturn indicates more tokens were returned than borrowed.
var ErrLimitReturn = errors.New("limit: discarding token, pool is full, returned multiple times")

// Limit controls the number of concurrent borrows.
type Limit struct {
	pool chan struct{}
}

// NewLimit creates a Limit that can borrow n elements concurrently.
func NewLimit(n int) Limit {
	return Limit{pool: make(chan struct{}, n)}
}

// Borrow borrows an element in blocking mode.
func (l Limit) Borrow() {
	l.pool <- struct{}{}
}

// Return returns a borrowed element, erroring when returning more than borrowed.
func (l Limit) Return() error {
	select {
	case <-l.pool:
		return nil
	default:
		return ErrLimitReturn
	}
}

// TryBorrow borrows an element without blocking.
func (l Limit) TryBorrow() bool {
	select {
	case l.pool <- struct{}{}:
		return true
	default:
		return false
	}
}
