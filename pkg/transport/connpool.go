// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package transport

import (
	"context"
	"net"
	"sync"
)

// ConnPool is a bounded pool of net.Conn connections for non-gRPC protocols.
// gRPC already multiplexes over HTTP/2, but a raw-TCP protocol benefits from
// this pool.
type ConnPool interface {
	// Get returns a pooled connection to network/addr, dialing a new one if
	// none is idle.
	Get(ctx context.Context, network, addr string) (net.Conn, error)
	// Put returns a healthy connection (previously obtained from Get) to the
	// pool, closing it if the pool is full.
	Put(conn net.Conn) error
	// Discard drops a connection without reusing it.
	Discard(conn net.Conn) error
	// Close closes all idle connections and clears the pool.
	Close() error
}

// connPool is a simple idle-connection pool keyed by "network/addr".
type connPool struct {
	mu      sync.Mutex
	idle    map[string][]net.Conn
	keyOf   map[net.Conn]string
	maxIdle int
}

// NewConnPool returns a ConnPool with the given maximum idle connections.
func NewConnPool(maxIdle int) ConnPool {
	if maxIdle < 1 {
		maxIdle = 1
	}
	return &connPool{
		idle:    make(map[string][]net.Conn),
		keyOf:   make(map[net.Conn]string),
		maxIdle: maxIdle,
	}
}

func (p *connPool) Get(ctx context.Context, network, addr string) (net.Conn, error) {
	key := network + "/" + addr

	p.mu.Lock()
	if conns := p.idle[key]; len(conns) > 0 {
		conn := conns[len(conns)-1]
		p.idle[key] = conns[:len(conns)-1]
		p.mu.Unlock()
		return conn, nil
	}
	p.mu.Unlock()

	var d net.Dialer
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.keyOf[conn] = key
	p.mu.Unlock()
	return conn, nil
}

func (p *connPool) Put(conn net.Conn) error {
	if conn == nil {
		return nil
	}

	p.mu.Lock()
	key, ok := p.keyOf[conn]
	if !ok {
		// A connection we did not hand out (or already returned): close it.
		p.mu.Unlock()
		return conn.Close()
	}

	if len(p.idle[key]) >= p.maxIdle {
		delete(p.keyOf, conn)
		p.mu.Unlock()
		return conn.Close()
	}
	p.idle[key] = append(p.idle[key], conn)
	p.mu.Unlock()
	return nil
}

func (p *connPool) Discard(conn net.Conn) error {
	if conn == nil {
		return nil
	}
	p.mu.Lock()
	delete(p.keyOf, conn)
	p.mu.Unlock()
	return conn.Close()
}

func (p *connPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, conns := range p.idle {
		for _, c := range conns {
			_ = c.Close()
		}
		delete(p.idle, key)
	}
	p.keyOf = make(map[net.Conn]string)
	return nil
}
