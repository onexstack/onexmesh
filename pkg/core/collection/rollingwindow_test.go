// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

import (
	"testing"
	"time"
)

func newIntBucket() *Bucket[int64] {
	return &Bucket[int64]{}
}

func TestRollingWindowAddAndReduce(t *testing.T) {
	w := NewRollingWindow(newIntBucket, 3, time.Second)

	for i := int64(1); i <= 10; i++ {
		w.Add(i)
	}

	var sum, count int64
	w.Reduce(func(b *Bucket[int64]) {
		sum += b.Sum
		count += b.Count
	})

	if count != 10 {
		t.Fatalf("count = %d, want 10", count)
	}
	if sum != 55 {
		t.Fatalf("sum = %d, want 55", sum)
	}
}

func TestRollingWindowIgnoreCurrentBucket(t *testing.T) {
	w := NewRollingWindow(newIntBucket, 3, time.Second, IgnoreCurrentBucket[int64, *Bucket[int64]]())

	w.Add(1)

	var count int64
	w.Reduce(func(b *Bucket[int64]) {
		count += b.Count
	})

	if count != 0 {
		t.Fatalf("count = %d, want 0 (current bucket ignored)", count)
	}
}

func TestRollingWindowBucketReset(t *testing.T) {
	b := &Bucket[int64]{}
	b.Add(3)
	b.Add(4)
	if b.Sum != 7 || b.Count != 2 {
		t.Fatalf("bucket = {sum=%d count=%d}, want {7 2}", b.Sum, b.Count)
	}
	b.Reset()
	if b.Sum != 0 || b.Count != 0 {
		t.Fatalf("bucket after reset = {sum=%d count=%d}, want {0 0}", b.Sum, b.Count)
	}
}
