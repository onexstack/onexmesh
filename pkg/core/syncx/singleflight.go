// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package syncx provides small concurrency primitives used across the core
// components: a single-flight group with freshness reporting and a spin lock
// for very short critical sections.
package syncx

import (
	"sync"
)

// SingleFlight lets concurrent calls with the same key share a single
// execution and its result, preventing duplicate work (cache stampede
// protection). Calls with different keys are independent.
type SingleFlight interface {
	Do(key string, fn func() (any, error)) (any, error)
	// DoEx is like Do but also reports whether this call actually executed fn
	// (fresh) or shared an in-flight result.
	DoEx(key string, fn func() (any, error)) (val any, fresh bool, err error)
}

type call struct {
	wg  sync.WaitGroup
	val any
	err error
}

type flightGroup struct {
	calls map[string]*call
	lock  sync.Mutex
}

// NewSingleFlight returns a SingleFlight.
func NewSingleFlight() SingleFlight {
	return &flightGroup{calls: make(map[string]*call)}
}

func (g *flightGroup) Do(key string, fn func() (any, error)) (any, error) {
	c, done := g.createCall(key)
	if done {
		return c.val, c.err
	}
	g.makeCall(c, key, fn)
	return c.val, c.err
}

func (g *flightGroup) DoEx(key string, fn func() (any, error)) (any, bool, error) {
	c, done := g.createCall(key)
	if done {
		return c.val, false, c.err
	}
	g.makeCall(c, key, fn)
	return c.val, true, c.err
}

func (g *flightGroup) createCall(key string) (c *call, done bool) {
	g.lock.Lock()
	if c, ok := g.calls[key]; ok {
		g.lock.Unlock()
		c.wg.Wait()
		return c, true
	}
	c = new(call)
	c.wg.Add(1)
	g.calls[key] = c
	g.lock.Unlock()
	return c, false
}

func (g *flightGroup) makeCall(c *call, key string, fn func() (any, error)) {
	defer func() {
		g.lock.Lock()
		delete(g.calls, key)
		g.lock.Unlock()
		c.wg.Done()
	}()
	c.val, c.err = fn()
}
