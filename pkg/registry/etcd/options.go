// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package etcd implements onexmesh registry.Registrar and registry.Discovery
// backed by etcd, using leases for automatic instance expiry (TTL) and prefix
// watch for change notifications. It serves as the reference implementation
// proving the registry abstraction is backend-agnostic.
package etcd

import "time"

// Options configures the Etcd backend.
type Options struct {
	// Endpoints are the etcd cluster addresses, e.g. ["127.0.0.1:2379"].
	Endpoints []string
	// DialTimeout bounds the initial connection attempt.
	DialTimeout time.Duration
	// Username and Password enable basic auth, when set.
	Username string
	Password string
	// TTL is the lease TTL in seconds; instances expire if not refreshed.
	TTL int64
	// Namespace scopes the registration keys.
	Namespace string
}
