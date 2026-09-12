// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package transport_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/onexstack/onexmesh/pkg/transport"
	_ "github.com/onexstack/onexmesh/pkg/transport/network/standard"
)

func TestGetStandardNetwork(t *testing.T) {
	n, err := transport.GetNetwork("standard")
	if err != nil {
		t.Fatal(err)
	}
	if n.String() != "standard" {
		t.Fatalf("String() = %q, want standard", n.String())
	}
}

func TestGetUnknownNetwork(t *testing.T) {
	if _, err := transport.GetNetwork("nope"); err == nil {
		t.Fatal("GetNetwork(unknown) should error")
	}
}

func TestConnPoolReuse(t *testing.T) {
	// A one-shot echo server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				io.Copy(c, c)
				c.Close()
			}(c)
		}
	}()

	addr := ln.Addr().String()
	pool := transport.NewConnPool(2)
	defer pool.Close()

	c1, err := pool.Get(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	_ = pool.Put(c1)

	// Second Get should reuse c1.
	c2, err := pool.Get(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	if c2 != c1 {
		t.Fatal("second Get did not reuse the pooled connection")
	}
	_ = pool.Put(c2)
}

func TestConnPoolDiscard(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	pool := transport.NewConnPool(1)
	defer pool.Close()

	conn, err := pool.Get(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Discard(conn); err != nil {
		t.Fatal(err)
	}

	select {
	case <-time.After(10 * time.Millisecond):
	}
}
