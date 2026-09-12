// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

import "sync"

// rebuildThreshold is the minimum live-entry count at which the SafeMap
// considers rebuilding its backing map after heavy deletions.
const rebuildThreshold = 1024

// SafeMap is a concurrency-safe generic map that avoids Go's map-delete memory
// retention (the backing array never shrinks on delete, see issue #20135). Once
// deletions outnumber live entries on a large map, the next Set rebuilds the
// backing map so the oversized allocation can be reclaimed.
type SafeMap[K comparable, V any] struct {
	lock      sync.RWMutex
	m         map[K]V
	deletions int
}

// NewSafeMap returns an empty SafeMap.
func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{m: make(map[K]V)}
}

// Get returns the value for key and whether it was present.
func (sm *SafeMap[K, V]) Get(key K) (V, bool) {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	v, ok := sm.m[key]
	return v, ok
}

// Set stores key -> value.
func (sm *SafeMap[K, V]) Set(key K, value V) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	sm.m[key] = value
	sm.maybeRebuildLocked()
}

// Del removes key from the map.
func (sm *SafeMap[K, V]) Del(key K) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	if _, ok := sm.m[key]; ok {
		delete(sm.m, key)
		sm.deletions++
	}
}

// Len returns the number of live entries.
func (sm *SafeMap[K, V]) Len() int {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	return len(sm.m)
}

// Range calls fn for each entry; iteration stops when fn returns false.
func (sm *SafeMap[K, V]) Range(fn func(key K, value V) bool) {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	for k, v := range sm.m {
		if !fn(k, v) {
			return
		}
	}
}

// maybeRebuildLocked rebuilds the backing map when deletions dominate a large
// map, so the oversized allocation can be reclaimed. The caller must hold the
// write lock.
func (sm *SafeMap[K, V]) maybeRebuildLocked() {
	if sm.deletions <= len(sm.m) || len(sm.m) < rebuildThreshold {
		return
	}
	nm := make(map[K]V, len(sm.m))
	for k, v := range sm.m {
		nm[k] = v
	}
	sm.m = nm
	sm.deletions = 0
}
