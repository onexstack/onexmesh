// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleFlightDedup(t *testing.T) {
	var calls atomic.Int32
	sf := NewSingleFlight()

	start := make(chan struct{})
	release := make(chan struct{})
	fn := func() (any, error) {
		calls.Add(1)
		<-release
		return 42, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			v, err := sf.Do("k", fn)
			if err != nil || v != 42 {
				t.Errorf("Do() = (%v, %v)", v, err)
			}
		}()
	}
	close(start)
	// Let every goroutine join the single in-flight call before releasing it.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("fn executed %d times, want 1", calls.Load())
	}
}

func TestSingleFlightDoEx(t *testing.T) {
	sf := NewSingleFlight()
	started := make(chan struct{})
	release := make(chan struct{})

	first := make(chan bool, 1)
	go func() {
		_, fresh, _ := sf.DoEx("k", func() (any, error) {
			close(started)
			<-release
			return 1, nil
		})
		first <- fresh
	}()

	<-started

	second := make(chan bool, 1)
	go func() {
		_, fresh, _ := sf.DoEx("k", func() (any, error) { return 2, nil })
		second <- fresh
	}()

	// Give the second caller time to join the in-flight call, then release the
	// first so both observe the shared result.
	time.Sleep(20 * time.Millisecond)
	close(release)

	if !<-first {
		t.Fatal("first call reported fresh=false, want true")
	}
	if <-second {
		t.Fatal("second call reported fresh=true, want false")
	}
}

func TestSingleFlightError(t *testing.T) {
	sf := NewSingleFlight()
	want := errors.New("boom")
	_, err := sf.Do("k", func() (any, error) { return nil, want })
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestSpinLock(t *testing.T) {
	var sl SpinLock
	var counter int
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sl.Lock()
			counter++
			sl.Unlock()
		}()
	}
	wg.Wait()
	if counter != 100 {
		t.Fatalf("counter = %d, want 100", counter)
	}
}
