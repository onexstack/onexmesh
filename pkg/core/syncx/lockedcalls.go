// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import "sync"

// LockedCalls ensures calls sharing a key run sequentially: a caller whose key
// is already in flight waits for the in-flight call to finish, then retries,
// so it always runs its own fn (unlike SingleFlight, which shares results).
type LockedCalls interface {
	Do(key string, fn func() (any, error)) (any, error)
}

type lockedGroup struct {
	mu sync.Mutex
	m  map[string]*sync.WaitGroup
}

// NewLockedCalls returns a LockedCalls.
func NewLockedCalls() LockedCalls {
	return &lockedGroup{m: make(map[string]*sync.WaitGroup)}
}

func (lg *lockedGroup) Do(key string, fn func() (any, error)) (any, error) {
begin:
	lg.mu.Lock()
	if wg, ok := lg.m[key]; ok {
		lg.mu.Unlock()
		wg.Wait()
		goto begin
	}
	return lg.makeCall(key, fn)
}

func (lg *lockedGroup) makeCall(key string, fn func() (any, error)) (any, error) {
	var wg sync.WaitGroup
	wg.Add(1)
	lg.m[key] = &wg
	lg.mu.Unlock()

	defer func() {
		// Delete the key before signaling Done: reversing the order would let a
		// concurrent Do see a not-yet-notified WaitGroup and block forever.
		lg.mu.Lock()
		delete(lg.m, key)
		lg.mu.Unlock()
		wg.Done()
	}()

	return fn()
}
