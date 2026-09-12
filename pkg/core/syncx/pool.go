// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package syncx

import (
	"sync"
	"time"
)

// PoolOption customizes a Pool.
type PoolOption func(*Pool)

// node is a link in the pool's idle list.
type node struct {
	item     any
	next     *node
	lastUsed time.Time
}

// Pool is a bounded resource pool. Unlike sync.Pool it caps the number of
// resources, can age resources out, and runs a custom destroy on them.
type Pool struct {
	limit   int
	created int
	maxAge  time.Duration
	lock    *sync.Mutex
	cond    *sync.Cond
	head    *node
	create  func() any
	destroy func(any)
}

// NewPool returns a Pool of at most n resources, creating them with create and
// destroying them with destroy. It panics on a non-positive size.
func NewPool(n int, create func() any, destroy func(any), opts ...PoolOption) *Pool {
	if n <= 0 {
		panic("syncx: pool size must be positive")
	}
	lock := &sync.Mutex{}
	p := &Pool{
		limit:   n,
		lock:    lock,
		cond:    sync.NewCond(lock),
		create:  create,
		destroy: destroy,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Get returns a resource, blocking until one is available.
func (p *Pool) Get() any {
	p.lock.Lock()
	defer p.lock.Unlock()

	for {
		if p.head != nil {
			head := p.head
			p.head = head.next
			if p.maxAge > 0 && time.Since(head.lastUsed) > p.maxAge {
				p.created--
				if p.destroy != nil {
					p.destroy(head.item)
				}
				continue
			}
			return head.item
		}

		if p.created < p.limit {
			p.created++
			return p.create()
		}

		p.cond.Wait()
	}
}

// Put returns a resource to the pool.
func (p *Pool) Put(x any) {
	if x == nil {
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	p.head = &node{item: x, next: p.head, lastUsed: time.Now()}
	p.cond.Signal()
}

// WithMaxAge returns a PoolOption that ages out idle resources older than d.
func WithMaxAge(d time.Duration) PoolOption {
	return func(p *Pool) { p.maxAge = d }
}
