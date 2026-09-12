// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package timex

import (
	"testing"
	"time"
)

func TestFakeTickerTickAndStop(t *testing.T) {
	ft := NewFakeTicker()
	ft.Tick()
	select {
	case <-ft.Chan():
	case <-time.After(time.Second):
		t.Fatal("Tick did not deliver")
	}

	ft.Stop()
	select {
	case <-ft.Done():
	case <-time.After(time.Second):
		t.Fatal("Stop did not close Done")
	}

	if err := ft.Wait(time.Millisecond); err != nil {
		t.Fatalf("Wait after Stop = %v, want nil", err)
	}
}

func TestFakeTickerWaitTimeout(t *testing.T) {
	ft := NewFakeTicker()
	if err := ft.Wait(10 * time.Millisecond); err != ErrTimeout {
		t.Fatalf("Wait() = %v, want ErrTimeout", err)
	}
}
