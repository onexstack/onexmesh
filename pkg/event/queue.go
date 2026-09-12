// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package event

import "sync"

// Queue is a bounded event buffer that retains the most recent events.
type Queue interface {
	// Push appends an event, dropping the oldest when capacity is exceeded.
	Push(e *Event)
	// Dump returns a snapshot of the buffered events.
	Dump() []*Event
}

type queue struct {
	mu       sync.Mutex
	events   []*Event
	capacity int
}

// NewQueue returns a Queue with the given capacity. A capacity <= 0 means
// unbounded.
func NewQueue(capacity int) Queue {
	return &queue{capacity: capacity}
}

func (q *queue) Push(e *Event) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.events = append(q.events, e)
	if q.capacity > 0 && len(q.events) > q.capacity {
		q.events = q.events[len(q.events)-q.capacity:]
	}
}

func (q *queue) Dump() []*Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*Event, len(q.events))
	copy(out, q.events)
	return out
}
