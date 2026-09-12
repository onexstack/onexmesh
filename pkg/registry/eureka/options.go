// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package eureka implements onexmesh registry.Registrar and registry.Discovery
// against a Netflix Eureka server over its REST API, using net/http only.
package eureka

import "time"

// Options configures the Eureka backend.
type Options struct {
	// ServerURL is the Eureka server base URL, e.g. "http://127.0.0.1:8761/eureka".
	ServerURL string
	// HostName is the hostname to register, defaulting to the local host.
	HostName string
	// Host is the instance IP/host advertised to clients.
	Host string
	// Port is the instance port advertised to clients.
	Port int
	// Protocol is the service protocol, e.g. "grpc" or "http".
	Protocol string
	// TTL is the lease renewal interval; the registrar heartbeats at half this
	// interval. Default 30s.
	TTL time.Duration
}
