// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package collection

import "sync"

// queueGrowThreshold is the capacity below which a Queue doubles on growth,
// mirroring Go's slice growth behavior.
const queueGrowThreshold = 256

// Queue is an auto-growing FIFO queue.
type Queue struct {
	lock     sync.Mutex
	elements []any
	head     int
	tail     int
	count    int
}

// NewQueue returns a Queue with the given initial capacity. It panics on a
// non-positive size.
func NewQueue(size int) *Queue {
	if size < 1 {
		panic("collection: queue size must be positive")
	}
	return &Queue{elements: make([]any, size)}
}

// Empty reports whether the queue has no elements.
func (q *Queue) Empty() bool {
	q.lock.Lock()
	empty := q.count == 0
	q.lock.Unlock()
	return empty
}

// Put appends element to the queue, growing it as needed.
func (q *Queue) Put(element any) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.count == len(q.elements) {
		nodes := make([]any, nextQueueCapacity(len(q.elements)))
		n := copy(nodes, q.elements[q.head:])
		copy(nodes[n:], q.elements[:q.head])
		q.head = 0
		q.tail = q.count
		q.elements = nodes
	}

	q.elements[q.tail] = element
	q.tail = (q.tail + 1) % len(q.elements)
	q.count++
}

// Take removes and returns the oldest element, or false if the queue is empty.
func (q *Queue) Take() (any, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.count == 0 {
		return nil, false
	}

	element := q.elements[q.head]
	q.elements[q.head] = nil
	q.head = (q.head + 1) % len(q.elements)
	q.count--

	return element, true
}

func nextQueueCapacity(capacity int) int {
	if capacity < queueGrowThreshold {
		return capacity << 1
	}
	// Double small queues, then grow toward 1.25x for large ones.
	return capacity + ((capacity + 3*queueGrowThreshold) >> 2)
}
