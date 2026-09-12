// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import "sync"

// DoneChan is a channel that can be closed multiple times safely.
type DoneChan struct {
	done chan struct{}
	once sync.Once
}

// NewDoneChan returns a DoneChan.
func NewDoneChan() *DoneChan {
	return &DoneChan{done: make(chan struct{})}
}

// Close closes the channel; it is safe to call more than once.
func (dc *DoneChan) Close() {
	dc.once.Do(func() { close(dc.done) })
}

// Done returns a channel that is closed when Close is called.
func (dc *DoneChan) Done() <-chan struct{} {
	return dc.done
}
