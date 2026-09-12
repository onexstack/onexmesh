// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package standard provides the standard-library Network implementation backed
// by net.Listen and net.Dial.
package standard

import (
	"net"

	"github.com/onexstack/onexmesh/pkg/transport"
)

func init() {
	transport.RegisterNetwork("standard", &network{})
}

type network struct{}

func (network) Listen(addr string) (net.Listener, error) { return net.Listen("tcp", addr) }
func (network) Dial(addr string) (net.Conn, error)       { return net.Dial("tcp", addr) }
func (network) String() string                           { return "standard" }
