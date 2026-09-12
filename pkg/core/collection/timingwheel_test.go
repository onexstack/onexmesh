// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

import (
	"testing"
	"time"

	"github.com/onexstack/onexmesh/pkg/core/timex"
)

func TestTimingWheelSetTimer(t *testing.T) {
	fired := make(chan any, 1)
	ft := timex.NewFakeTicker()
	tw, err := NewTimingWheelWithTicker(time.Millisecond, 10, func(_, value any) {
		fired <- value
	}, ft)
	if err != nil {
		t.Fatal(err)
	}
	defer tw.Stop()

	if err := tw.SetTimer("k", "v", 3*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	drive(ft, 10)

	select {
	case v := <-fired:
		if v != "v" {
			t.Fatalf("fired = %v, want v", v)
		}
	case <-time.After(time.Second):
		t.Fatal("timer did not fire")
	}
}

func TestTimingWheelRemoveTimer(t *testing.T) {
	fired := make(chan any, 1)
	ft := timex.NewFakeTicker()
	tw, _ := NewTimingWheelWithTicker(time.Millisecond, 10, func(_, value any) {
		fired <- value
	}, ft)
	defer tw.Stop()

	if err := tw.SetTimer("k", "v", 3*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := tw.RemoveTimer("k"); err != nil {
		t.Fatal(err)
	}
	drive(ft, 10)

	select {
	case <-fired:
		t.Fatal("timer fired after removal")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTimingWheelDrain(t *testing.T) {
	ft := timex.NewFakeTicker()
	tw, _ := NewTimingWheelWithTicker(time.Millisecond, 10, func(_, _ any) {}, ft)
	defer tw.Stop()

	if err := tw.SetTimer("k", "v", time.Second); err != nil {
		t.Fatal(err)
	}
	drained := make(chan struct{})
	if err := tw.Drain(func(key, value any) {
		if key != "k" || value != "v" {
			t.Errorf("drain(%v, %v)", key, value)
		}
		close(drained)
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("drain did not run")
	}
}

// drive ticks ft n times, giving the wheel's goroutine a chance to advance.
func drive(ft *timex.FakeTicker, n int) {
	for i := 0; i < n; i++ {
		ft.Tick()
		time.Sleep(time.Millisecond)
	}
}
