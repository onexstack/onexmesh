// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package transport

import (
	"fmt"
	"net"
)

// Network abstracts the low-level socket layer (listen and dial) so a future
// epoll/netpoll backend can be swapped in without touching the upper layers.
type Network interface {
	// Listen returns a listener bound to addr.
	Listen(addr string) (net.Listener, error)
	// Dial connects to addr.
	Dial(addr string) (net.Conn, error)
	// String names the network implementation.
	String() string
}

// The network registry table. Writes happen only from init() during package
// initialization; reads happen at runtime.
var networks = map[string]Network{}

// RegisterNetwork registers a Network under name. Backend packages call this
// from init().
func RegisterNetwork(name string, n Network) {
	networks[name] = n
}

// GetNetwork returns a Network by name.
func GetNetwork(name string) (Network, error) {
	n, ok := networks[name]
	if !ok {
		return nil, fmt.Errorf("transport: network %q not registered", name)
	}
	return n, nil
}
