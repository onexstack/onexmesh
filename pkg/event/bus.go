// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package event provides a lightweight in-process event bus and queue, modeled
// on kitex's pkg/event. It decouples producers from consumers: for example, a
// service-discovery cache can dispatch a "discovery.change" event that a
// circuit-breaker suite subscribes to in order to evict state for removed
// instances.
package event

import (
	"reflect"
	"sync"
)

// Callback handles a dispatched event.
type Callback func(*Event)

// Event is a named payload delivered on a Bus.
type Event struct {
	Name string
	Data any
}

// Bus dispatches named events to registered callbacks.
type Bus interface {
	// Watch registers cb for events named name.
	Watch(name string, cb Callback)
	// Unwatch removes a previously registered cb.
	Unwatch(name string, cb Callback)
	// Dispatch delivers e to all callbacks watching its name.
	Dispatch(e *Event)
}

type bus struct {
	mu       sync.RWMutex
	watchers map[string]map[uintptr]Callback
}

// NewBus returns an empty Bus.
func NewBus() Bus {
	return &bus{watchers: map[string]map[uintptr]Callback{}}
}

func (b *bus) Watch(name string, cb Callback) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.watchers[name] == nil {
		b.watchers[name] = map[uintptr]Callback{}
	}
	b.watchers[name][callbackID(cb)] = cb
}

func (b *bus) Unwatch(name string, cb Callback) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if m := b.watchers[name]; m != nil {
		delete(m, callbackID(cb))
	}
}

func (b *bus) Dispatch(e *Event) {
	b.mu.RLock()
	// Copy the callbacks so they can be invoked without holding the lock.
	cbs := make([]Callback, 0, len(b.watchers[e.Name]))
	for _, cb := range b.watchers[e.Name] {
		cbs = append(cbs, cb)
	}
	b.mu.RUnlock()

	for _, cb := range cbs {
		cb(e)
	}
}

// callbackID returns a comparable key for a callback.
func callbackID(cb Callback) uintptr {
	return reflect.ValueOf(cb).Pointer()
}
