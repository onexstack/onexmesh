// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package timex provides a Ticker abstraction with a fake implementation for
// deterministic, clock-injectable tests of periodic logic.
package timex

import (
	"errors"
	"sync"
	"time"
)

// ErrTimeout is returned by FakeTicker.Wait when the deadline elapses first.
var ErrTimeout = errors.New("timex: wait timeout")

// Ticker emits periodic ticks.
type Ticker interface {
	// Chan returns the tick channel.
	Chan() <-chan time.Time
	// Stop stops the ticker.
	Stop()
}

// ticker adapts *time.Ticker to Ticker.
type ticker struct {
	*time.Ticker
}

// NewTicker returns a real Ticker with the given interval.
func NewTicker(d time.Duration) Ticker {
	return &ticker{Ticker: time.NewTicker(d)}
}

func (t *ticker) Chan() <-chan time.Time { return t.C }

// FakeTicker is a manually-driven Ticker for tests.
type FakeTicker struct {
	c    chan time.Time
	done chan struct{}
	once sync.Once
}

// NewFakeTicker returns a FakeTicker that must be advanced with Tick.
func NewFakeTicker() *FakeTicker {
	return &FakeTicker{
		c:    make(chan time.Time, 1),
		done: make(chan struct{}),
	}
}

// Chan returns the tick channel.
func (ft *FakeTicker) Chan() <-chan time.Time { return ft.c }

// Stop closes the Done channel; it is safe to call multiple times.
func (ft *FakeTicker) Stop() {
	ft.once.Do(func() { close(ft.done) })
}

// Done returns a channel closed when Stop is called.
func (ft *FakeTicker) Done() <-chan struct{} { return ft.done }

// Tick emits a single tick.
func (ft *FakeTicker) Tick() {
	select {
	case ft.c <- time.Now():
	case <-ft.done:
	}
}

// Wait blocks until Stop is called or the duration elapses, whichever first.
func (ft *FakeTicker) Wait(d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ft.done:
		return nil
	case <-t.C:
		return ErrTimeout
	}
}
