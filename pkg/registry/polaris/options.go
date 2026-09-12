// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package polaris implements onexmesh registry.Registrar and registry.Discovery
// backed by the Polaris service mesh (github.com/polarismesh/polaris-go).
package polaris

// Options configures the Polaris backend.
type Options struct {
	// Addr is the Polaris server address, e.g. "127.0.0.1:8091".
	Addr string
	// Namespace is the Polaris namespace to register/discover in.
	Namespace string
	// Token is the service access token, if required by the server.
	Token string
	// TTL is the heartbeat TTL in seconds; meaningful only when Heartbeat is true.
	TTL int
	// Heartbeat enables SDK-managed auto heartbeat (requires TTL).
	Heartbeat bool
	// Protocol is the service protocol, e.g. "grpc" or "http".
	Protocol string
	// Host is the local instance host used for registration.
	Host string
	// Port is the local instance port used for registration.
	Port int
	// Version is the service version.
	Version string
	// Metadata carries arbitrary instance metadata.
	Metadata map[string]string
}
