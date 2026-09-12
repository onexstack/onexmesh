// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package collection provides concurrency-safe generic collections used by the
// resilience primitives: a rolling window for time-bucketed statistics and a
// safe map that avoids the Go map-delete memory-growth issue.
package collection

import (
	"sync"
	"time"
)

// Numerical is a constraint that permits any numeric type.
type Numerical interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// BucketInterface is the interface that buckets must satisfy.
type BucketInterface[T Numerical] interface {
	Add(v T)
	Reset()
}

// RollingWindowOption customizes a RollingWindow.
type RollingWindowOption[T Numerical, B BucketInterface[T]] func(*RollingWindow[T, B])

// RollingWindow keeps a fixed number of time buckets and aggregates the values
// added to them, sliding the bucket boundary forward as time advances.
type RollingWindow[T Numerical, B BucketInterface[T]] struct {
	lock          sync.RWMutex
	size          int
	win           *window[T, B]
	interval      time.Duration
	offset        int
	ignoreCurrent bool
	lastTime      time.Time
}

// NewRollingWindow returns a RollingWindow with size buckets each covering
// interval. newBucket creates the concrete bucket instances.
func NewRollingWindow[T Numerical, B BucketInterface[T]](newBucket func() B, size int,
	interval time.Duration, opts ...RollingWindowOption[T, B]) *RollingWindow[T, B] {
	if size < 1 {
		panic("collection: rolling window size must be greater than 0")
	}

	w := &RollingWindow[T, B]{
		size:     size,
		win:      newWindow(newBucket, size),
		interval: interval,
		lastTime: time.Now(),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Add adds a value to the current bucket.
func (rw *RollingWindow[T, B]) Add(v T) {
	rw.lock.Lock()
	defer rw.lock.Unlock()
	rw.updateOffset()
	rw.win.add(rw.offset, v)
}

// Reduce runs fn on all buckets, ignoring the current bucket if IgnoreCurrentBucket
// was set.
func (rw *RollingWindow[T, B]) Reduce(fn func(b B)) {
	rw.lock.RLock()
	defer rw.lock.RUnlock()

	var diff int
	span := rw.span()
	// Ignore the current bucket because it holds partial data.
	if span == 0 && rw.ignoreCurrent {
		diff = rw.size - 1
	} else {
		diff = rw.size - span
	}
	if diff > 0 {
		offset := (rw.offset + span + 1) % rw.size
		rw.win.reduce(offset, diff, fn)
	}
}

// span returns how many buckets have elapsed since lastTime, clamped to size.
func (rw *RollingWindow[T, B]) span() int {
	offset := int(time.Since(rw.lastTime) / rw.interval)
	if 0 <= offset && offset < rw.size {
		return offset
	}
	return rw.size
}

func (rw *RollingWindow[T, B]) updateOffset() {
	span := rw.span()
	if span <= 0 {
		return
	}

	offset := rw.offset
	for i := 0; i < span; i++ {
		rw.win.resetBucket((offset + i + 1) % rw.size)
	}

	rw.offset = (offset + span) % rw.size
	now := time.Now()
	// Align lastTime to the interval boundary.
	rw.lastTime = now.Add(-(now.Sub(rw.lastTime) % rw.interval))
}

// IgnoreCurrentBucket makes Reduce ignore the current (partial) bucket.
func IgnoreCurrentBucket[T Numerical, B BucketInterface[T]]() RollingWindowOption[T, B] {
	return func(w *RollingWindow[T, B]) {
		w.ignoreCurrent = true
	}
}

// Bucket holds the sum and count of the additions.
type Bucket[T Numerical] struct {
	Sum   T
	Count int64
}

// Add adds v to the bucket sum.
func (b *Bucket[T]) Add(v T) {
	b.Sum += v
	b.Count++
}

// Reset clears the bucket.
func (b *Bucket[T]) Reset() {
	b.Sum = 0
	b.Count = 0
}

type window[T Numerical, B BucketInterface[T]] struct {
	buckets []B
	size    int
}

func newWindow[T Numerical, B BucketInterface[T]](newBucket func() B, size int) *window[T, B] {
	buckets := make([]B, size)
	for i := 0; i < size; i++ {
		buckets[i] = newBucket()
	}
	return &window[T, B]{buckets: buckets, size: size}
}

func (w *window[T, B]) add(offset int, v T) {
	w.buckets[offset%w.size].Add(v)
}

func (w *window[T, B]) reduce(start, count int, fn func(b B)) {
	for i := 0; i < count; i++ {
		fn(w.buckets[(start+i)%w.size])
	}
}

func (w *window[T, B]) resetBucket(offset int) {
	w.buckets[offset%w.size].Reset()
}
