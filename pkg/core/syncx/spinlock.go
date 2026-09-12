// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import (
	"runtime"
	"sync/atomic"
)

// SpinLock is a CAS-based spin lock for very short critical sections where the
// cost of a mutex's goroutine suspension outweighs a few busy iterations.
type SpinLock struct {
	lock uint32
}

// Lock spins until the lock is acquired.
func (sl *SpinLock) Lock() {
	for !sl.TryLock() {
		runtime.Gosched()
	}
}

// TryLock attempts to acquire the lock without blocking.
func (sl *SpinLock) TryLock() bool {
	return atomic.CompareAndSwapUint32(&sl.lock, 0, 1)
}

// Unlock releases the lock.
func (sl *SpinLock) Unlock() {
	atomic.StoreUint32(&sl.lock, 0)
}
