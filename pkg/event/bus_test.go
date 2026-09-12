// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package event

import "testing"

func TestBusDispatch(t *testing.T) {
	b := NewBus()
	got := make(chan *Event, 1)
	b.Watch("change", func(e *Event) { got <- e })

	b.Dispatch(&Event{Name: "change", Data: 42})
	select {
	case e := <-got:
		if e.Data != 42 {
			t.Fatalf("Data = %v, want 42", e.Data)
		}
	default:
		t.Fatal("callback not invoked")
	}
}

func TestBusUnwatch(t *testing.T) {
	b := NewBus()
	var called int
	cb := func(*Event) { called++ }
	b.Watch("change", cb)
	b.Unwatch("change", cb)
	b.Dispatch(&Event{Name: "change"})
	if called != 0 {
		t.Fatalf("called = %d, want 0", called)
	}
}

func TestBusUnrelatedName(t *testing.T) {
	b := NewBus()
	called := false
	b.Watch("change", func(*Event) { called = true })
	b.Dispatch(&Event{Name: "other"})
	if called {
		t.Fatal("callback invoked for unrelated event name")
	}
}
