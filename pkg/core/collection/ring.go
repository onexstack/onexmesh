// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

import "sync"

// Ring is a fixed-size ring buffer. It overwrites the oldest element once full.
type Ring struct {
	elements []any
	index    int
	lock     sync.RWMutex
}

// NewRing returns a Ring of size n. It panics on a non-positive size.
func NewRing(n int) *Ring {
	if n < 1 {
		panic("collection: ring size must be positive")
	}
	return &Ring{elements: make([]any, n)}
}

// Add appends v to the ring, overwriting the oldest element when full.
func (r *Ring) Add(v any) {
	r.lock.Lock()
	defer r.lock.Unlock()

	rlen := len(r.elements)
	r.elements[r.index%rlen] = v
	r.index++

	// Prevent the index from overflowing on a very long-lived ring.
	if r.index >= rlen<<1 {
		r.index -= rlen
	}
}

// Take returns the ring's elements in insertion order (oldest first).
func (r *Ring) Take() []any {
	r.lock.RLock()
	defer r.lock.RUnlock()

	rlen := len(r.elements)
	var size, start int
	if r.index > rlen {
		size = rlen
		start = r.index % rlen
	} else {
		size = r.index
	}

	elements := make([]any, size)
	for i := 0; i < size; i++ {
		elements[i] = r.elements[(start+i)%rlen]
	}
	return elements
}
