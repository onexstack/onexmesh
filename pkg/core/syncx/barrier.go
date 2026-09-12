// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import "sync"

// Barrier is a mutex guarding a shared resource.
type Barrier struct {
	lock sync.Mutex
}

// Guard runs fn while holding the barrier's lock.
func (b *Barrier) Guard(fn func()) {
	Guard(&b.lock, fn)
}

// Guard runs fn while holding lock.
func Guard(lock sync.Locker, fn func()) {
	lock.Lock()
	defer lock.Unlock()
	fn()
}
