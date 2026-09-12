// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

import (
	"errors"
	"testing"
	"time"
)

func TestRingOverwrite(t *testing.T) {
	r := NewRing(3)
	r.Add(1)
	r.Add(2)
	r.Add(3)
	r.Add(4) // overwrites 1

	got := r.Take()
	want := []any{2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("Take() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Take() = %v, want %v", got, want)
		}
	}
}

func TestQueueFIFO(t *testing.T) {
	q := NewQueue(2)
	q.Put(1)
	q.Put(2)
	q.Put(3) // triggers growth

	if v, ok := q.Take(); !ok || v != 1 {
		t.Fatalf("Take() = %v, %v, want 1, true", v, ok)
	}
	if v, ok := q.Take(); !ok || v != 2 {
		t.Fatalf("Take() = %v, %v, want 2, true", v, ok)
	}
	if v, ok := q.Take(); !ok || v != 3 {
		t.Fatalf("Take() = %v, %v, want 3, true", v, ok)
	}
	if !q.Empty() {
		t.Fatal("queue should be empty")
	}
}

func TestCacheSetGet(t *testing.T) {
	c, err := NewCache(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	c.Set("k", "v")
	if v, ok := c.Get("k"); !ok || v != "v" {
		t.Fatalf("Get(k) = %v, %v, want v, true", v, ok)
	}
}

func TestCacheTakeFetchesOnce(t *testing.T) {
	c, err := NewCache(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	calls := 0
	for i := 0; i < 3; i++ {
		v, err := c.Take("k", func() (any, error) {
			calls++
			return "v", nil
		})
		if err != nil || v != "v" {
			t.Fatalf("Take() = %v, %v", v, err)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
}

func TestCacheTakePropagatesError(t *testing.T) {
	c, err := NewCache(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	want := errors.New("boom")
	if _, err := c.Take("k", func() (any, error) { return nil, want }); !errors.Is(err, want) {
		t.Fatalf("Take() err = %v, want %v", err, want)
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c, err := NewCache(time.Minute, WithLimit(2))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // evicts "a"

	if _, ok := c.Get("a"); ok {
		t.Fatal("a should have been evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("b should remain")
	}
}
