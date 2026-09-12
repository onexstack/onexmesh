// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package nacos implements onexmesh registry.Registrar and registry.Discovery
// backed by the Nacos service discovery (github.com/nacos-group/nacos-sdk-go).
package nacos

// Options configures the Nacos backend.
type Options struct {
	// Addr is the Nacos server address, e.g. "127.0.0.1:8848".
	Addr string
	// ContextPath is the Nacos server context path, e.g. "/nacos".
	ContextPath string
	// NamespaceID is the Nacos namespace ID. Empty means the public namespace.
	NamespaceID string
	// GroupName is the service group name. Default "DEFAULT_GROUP".
	GroupName string
	// ClusterName is the service cluster name. Default "DEFAULT".
	ClusterName string
	// Username and Password enable Nacos auth when set.
	Username string
	Password string
	// Host is the local instance host used for registration.
	Host string
	// Port is the local instance port used for registration.
	Port int
	// Weight is the instance weight. Default 100.
	Weight float64
	// Protocol is the service protocol, e.g. "grpc" or "http".
	Protocol string
}
