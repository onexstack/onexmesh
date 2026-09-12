// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package transport

// This file defines a message-level Socket abstraction (modeled after
// go-micro's transport.Socket) reserved for future non-HTTP/2 transports such
// as QUIC or a custom multiplexed binary protocol. The current gRPC and HTTP
// servers do not use it; it exists so such a transport can be added without
// touching pkg/server or pkg/client.

// SocketMessage is a frame with a header map and a raw body.
type SocketMessage struct {
	Header map[string]string
	Body   []byte
}

// Socket is a bidirectional message stream over an established connection.
type Socket interface {
	// Recv reads the next message.
	Recv(*SocketMessage) error
	// Send writes a message.
	Send(*SocketMessage) error
	// Close closes the socket.
	Close() error
	// Local returns the local address.
	Local() string
	// Remote returns the remote address.
	Remote() string
}
