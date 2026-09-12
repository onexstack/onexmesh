// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package transport

import (
	"context"
	"net"
)

// TransHandler is a connection-level handler abstraction (modeled after
// kitex's remote.TransHandler) reserved for future custom RPC protocols. It
// pairs with SocketMessage to describe how a framed message is read/written on
// a raw net.Conn and how connection lifecycle events are observed. The current
// gRPC and HTTP transports do not use it.
type TransHandler interface {
	// Write encodes and writes a message to conn.
	Write(ctx context.Context, conn net.Conn, m SocketMessage) error
	// Read reads and decodes a message from conn.
	Read(ctx context.Context, conn net.Conn, m SocketMessage) error
	// OnActive is called when a connection becomes active, returning a
	// connection-scoped context.
	OnActive(ctx context.Context, conn net.Conn) (context.Context, error)
	// OnInactive is called when a connection closes.
	OnInactive(ctx context.Context, conn net.Conn)
	// OnError is called on a connection error.
	OnError(ctx context.Context, err error, conn net.Conn)
	// OnMessage is called for each received message.
	OnMessage(ctx context.Context, args, result SocketMessage) (context.Context, error)
}
