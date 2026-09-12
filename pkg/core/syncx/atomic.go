// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import (
	"math"
	"sync/atomic"
	"time"
)

// AtomicBool is an atomic boolean.
type AtomicBool uint32

// NewAtomicBool returns a zero AtomicBool.
func NewAtomicBool() *AtomicBool { return new(AtomicBool) }

// ForAtomicBool returns an AtomicBool initialized to val.
func ForAtomicBool(val bool) *AtomicBool {
	b := NewAtomicBool()
	b.Set(val)
	return b
}

// CompareAndSwap swaps if the current value equals old.
func (b *AtomicBool) CompareAndSwap(old, val bool) bool {
	var ov, nv uint32
	if old {
		ov = 1
	}
	if val {
		nv = 1
	}
	return atomic.CompareAndSwapUint32((*uint32)(b), ov, nv)
}

// Set sets the value.
func (b *AtomicBool) Set(v bool) {
	if v {
		atomic.StoreUint32((*uint32)(b), 1)
	} else {
		atomic.StoreUint32((*uint32)(b), 0)
	}
}

// True reports whether the value is true.
func (b *AtomicBool) True() bool {
	return atomic.LoadUint32((*uint32)(b)) == 1
}

// AtomicInt32 is an atomic int32.
type AtomicInt32 int32

// NewAtomicInt32 returns a zero AtomicInt32.
func NewAtomicInt32() *AtomicInt32 { return new(AtomicInt32) }

// ForAtomicInt32 returns an AtomicInt32 initialized to val.
func ForAtomicInt32(val int32) *AtomicInt32 {
	i := NewAtomicInt32()
	i.Set(val)
	return i
}

// Add adds delta and returns the new value.
func (i *AtomicInt32) Add(delta int32) int32 {
	return atomic.AddInt32((*int32)(i), delta)
}

// Load returns the current value.
func (i *AtomicInt32) Load() int32 { return atomic.LoadInt32((*int32)(i)) }

// Set sets the value.
func (i *AtomicInt32) Set(val int32) { atomic.StoreInt32((*int32)(i), val) }

// CompareAndSwap swaps if the current value equals old.
func (i *AtomicInt32) CompareAndSwap(old, val int32) bool {
	return atomic.CompareAndSwapInt32((*int32)(i), old, val)
}

// AtomicInt64 is an atomic int64.
type AtomicInt64 int64

// NewAtomicInt64 returns a zero AtomicInt64.
func NewAtomicInt64() *AtomicInt64 { return new(AtomicInt64) }

// ForAtomicInt64 returns an AtomicInt64 initialized to val.
func ForAtomicInt64(val int64) *AtomicInt64 {
	i := NewAtomicInt64()
	i.Set(val)
	return i
}

// Add adds delta and returns the new value.
func (i *AtomicInt64) Add(delta int64) int64 {
	return atomic.AddInt64((*int64)(i), delta)
}

// Load returns the current value.
func (i *AtomicInt64) Load() int64 { return atomic.LoadInt64((*int64)(i)) }

// Set sets the value.
func (i *AtomicInt64) Set(val int64) { atomic.StoreInt64((*int64)(i), val) }

// CompareAndSwap swaps if the current value equals old.
func (i *AtomicInt64) CompareAndSwap(old, val int64) bool {
	return atomic.CompareAndSwapInt64((*int64)(i), old, val)
}

// AtomicFloat64 is an atomic float64.
type AtomicFloat64 uint64

// NewAtomicFloat64 returns a zero AtomicFloat64.
func NewAtomicFloat64() *AtomicFloat64 { return new(AtomicFloat64) }

// ForAtomicFloat64 returns an AtomicFloat64 initialized to val.
func ForAtomicFloat64(val float64) *AtomicFloat64 {
	f := NewAtomicFloat64()
	f.Set(val)
	return f
}

// Add adds val and returns the new value.
func (f *AtomicFloat64) Add(val float64) float64 {
	for {
		old := f.Load()
		nv := old + val
		if f.CompareAndSwap(old, nv) {
			return nv
		}
	}
}

// CompareAndSwap swaps if the current value equals old.
func (f *AtomicFloat64) CompareAndSwap(old, val float64) bool {
	return atomic.CompareAndSwapUint64((*uint64)(f), math.Float64bits(old), math.Float64bits(val))
}

// Load returns the current value.
func (f *AtomicFloat64) Load() float64 {
	return math.Float64frombits(atomic.LoadUint64((*uint64)(f)))
}

// Set sets the value.
func (f *AtomicFloat64) Set(val float64) {
	atomic.StoreUint64((*uint64)(f), math.Float64bits(val))
}

// AtomicDuration is an atomic time.Duration.
type AtomicDuration int64

// NewAtomicDuration returns a zero AtomicDuration.
func NewAtomicDuration() *AtomicDuration { return new(AtomicDuration) }

// ForAtomicDuration returns an AtomicDuration initialized to val.
func ForAtomicDuration(val time.Duration) *AtomicDuration {
	d := NewAtomicDuration()
	d.Set(val)
	return d
}

// CompareAndSwap swaps if the current value equals old.
func (d *AtomicDuration) CompareAndSwap(old, val time.Duration) bool {
	return atomic.CompareAndSwapInt64((*int64)(d), int64(old), int64(val))
}

// Load returns the current value.
func (d *AtomicDuration) Load() time.Duration {
	return time.Duration(atomic.LoadInt64((*int64)(d)))
}

// Set sets the value.
func (d *AtomicDuration) Set(val time.Duration) {
	atomic.StoreInt64((*int64)(d), int64(val))
}
