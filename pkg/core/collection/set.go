// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

// Set is a concurrency-unsafe generic set. Guard it externally when shared
// across goroutines.
type Set[T comparable] map[T]struct{}

// NewSet returns an empty Set.
func NewSet[T comparable]() Set[T] {
	return make(Set[T])
}

// Add inserts v into the set.
func (s Set[T]) Add(v T) {
	s[v] = struct{}{}
}

// Remove deletes v from the set.
func (s Set[T]) Remove(v T) {
	delete(s, v)
}

// Contains reports whether v is in the set.
func (s Set[T]) Contains(v T) bool {
	_, ok := s[v]
	return ok
}

// Len returns the number of elements.
func (s Set[T]) Len() int {
	return len(s)
}

// Values returns the elements in an unspecified order.
func (s Set[T]) Values() []T {
	out := make([]T, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	return out
}
