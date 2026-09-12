// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package consul implements onexmesh registry.Registrar and registry.Discovery
// backed by HashiCorp Consul (github.com/hashicorp/consul/api).
package consul

import "time"

// Options configures the Consul backend.
type Options struct {
	// Addr is the Consul agent address, e.g. "127.0.0.1:8500".
	Addr string
	// Scheme is the URI scheme of the Consul agent, "http" or "https".
	Scheme string
	// Token is the Consul ACL token, if required.
	Token string
	// Datacenter selects the Consul datacenter.
	Datacenter string
	// Namespace is the Consul Enterprise namespace.
	Namespace string
	// TTL is the service check TTL; the registrar heartbeats at half this
	// interval. Default 15s.
	TTL time.Duration
	// Host is the local instance host used for registration.
	Host string
	// Port is the local instance port used for registration.
	Port int
	// Protocol is the service protocol, e.g. "grpc" or "http".
	Protocol string
}
