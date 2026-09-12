// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package event

import "testing"

func TestQueueBounded(t *testing.T) {
	q := NewQueue(2)
	q.Push(&Event{Name: "a"})
	q.Push(&Event{Name: "b"})
	q.Push(&Event{Name: "c"})

	got := q.Dump()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "b" || got[1].Name != "c" {
		t.Fatalf("events = %v, want [b c]", got)
	}
}

func TestQueueUnbounded(t *testing.T) {
	q := NewQueue(0)
	for i := 0; i < 10; i++ {
		q.Push(&Event{})
	}
	if got := q.Dump(); len(got) != 10 {
		t.Fatalf("len = %d, want 10", len(got))
	}
}
