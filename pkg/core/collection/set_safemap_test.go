// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

import "testing"

func TestSet(t *testing.T) {
	s := NewSet[string]()
	s.Add("a")
	s.Add("b")
	s.Add("a") // duplicate

	if !s.Contains("a") || !s.Contains("b") || s.Contains("c") {
		t.Fatalf("contains: a=%v b=%v c=%v", s.Contains("a"), s.Contains("b"), s.Contains("c"))
	}
	if s.Len() != 2 {
		t.Fatalf("len = %d, want 2", s.Len())
	}

	s.Remove("a")
	if s.Contains("a") || s.Len() != 1 {
		t.Fatalf("after remove: contains(a)=%v len=%d", s.Contains("a"), s.Len())
	}
}

func TestSafeMap(t *testing.T) {
	m := NewSafeMap[string, int]()
	if _, ok := m.Get("k"); ok {
		t.Fatal("Get on empty map = ok")
	}

	m.Set("k", 1)
	if v, ok := m.Get("k"); !ok || v != 1 {
		t.Fatalf("Get = (%d, %v), want (1, true)", v, ok)
	}

	m.Del("k")
	if _, ok := m.Get("k"); ok {
		t.Fatal("Get after Del = ok")
	}

	m.Set("a", 1)
	m.Set("b", 2)
	sum := 0
	m.Range(func(_ string, v int) bool {
		sum += v
		return true
	})
	if sum != 3 {
		t.Fatalf("Range sum = %d, want 3", sum)
	}
}
