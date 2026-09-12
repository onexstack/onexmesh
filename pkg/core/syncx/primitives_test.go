// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import (
	"sync"
	"testing"
	"time"
)

func TestPoolReusesResources(t *testing.T) {
	created := 0
	p := NewPool(2, func() any {
		created++
		return &struct{}{}
	}, func(any) {})

	a := p.Get()
	p.Put(a)
	_ = p.Get()

	if created != 1 {
		t.Fatalf("created = %d, want 1 (second Get should reuse)", created)
	}
}

func TestPoolBlocksAtLimit(t *testing.T) {
	p := NewPool(1, func() any { return 1 }, func(any) {})
	_ = p.Get() // exhaust the pool

	done := make(chan struct{})
	go func() {
		_ = p.Get()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Get() returned before a resource was freed")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestTimeoutLimitBorrowTimeout(t *testing.T) {
	l := NewTimeoutLimit(1)
	_ = l.TryBorrow()

	if err := l.Borrow(10 * time.Millisecond); err != ErrTimeout {
		t.Fatalf("Borrow() err = %v, want ErrTimeout", err)
	}
}

func TestTimeoutLimitReturnWakesBorrower(t *testing.T) {
	l := NewTimeoutLimit(1)
	if !l.TryBorrow() {
		t.Fatal("first TryBorrow should succeed")
	}

	done := make(chan struct{})
	go func() {
		if err := l.Borrow(time.Second); err != nil {
			t.Errorf("Borrow() err = %v", err)
		}
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	if err := l.Return(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Borrower was not woken after Return")
	}
}

func TestAtomicBool(t *testing.T) {
	b := NewAtomicBool()
	if b.True() {
		t.Fatal("new AtomicBool should be false")
	}
	b.Set(true)
	if !b.True() {
		t.Fatal("AtomicBool should be true after Set(true)")
	}
	if !b.CompareAndSwap(true, false) {
		t.Fatal("CAS(true, false) should succeed")
	}
	if b.True() {
		t.Fatal("AtomicBool should be false after CAS")
	}
}

func TestAtomicFloat64Add(t *testing.T) {
	f := NewAtomicFloat64()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.Add(1)
		}()
	}
	wg.Wait()
	if got := f.Load(); got != 100 {
		t.Fatalf("sum = %v, want 100", got)
	}
}

func TestLockedCallsSerialize(t *testing.T) {
	lc := NewLockedCalls()
	var active, maxActive int32
	var mu sync.Mutex

	run := func() {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		time.Sleep(time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = lc.Do("k", func() (any, error) { run(); return nil, nil })
		}()
	}
	wg.Wait()

	if maxActive != 1 {
		t.Fatalf("max concurrent = %d, want 1 (serialized)", maxActive)
	}
}

func TestBarrierGuard(t *testing.T) {
	var b Barrier
	var count int
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Guard(func() { count++ })
		}()
	}
	wg.Wait()
	if count != 10 {
		t.Fatalf("count = %d, want 10", count)
	}
}
