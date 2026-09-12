// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package registry defines the service registration and discovery abstraction
// for onexmesh. It strictly separates the server-side Registrar from the
// client-side Discovery (interface segregation), and plugs concrete backends
// (Polaris, Etcd, Kubernetes, ...) in through a factory registry.
package registry

import "context"

// ServiceInstance describes a single service instance as seen by a registry.
type ServiceInstance struct {
	// ID uniquely identifies the instance (e.g. hostname, or the backend's
	// assigned instance id). May be empty for backends that derive it.
	ID string
	// Name is the logical service name, e.g. "edu.course.student-api".
	Name string
	// Version is the service version.
	Version string
	// Metadata carries arbitrary key/value tags.
	Metadata map[string]string
	// Endpoints lists the reachable addresses with scheme prefixes, e.g.
	// ["grpc://127.0.0.1:39090", "http://127.0.0.1:8080"].
	Endpoints []string
}

// Registrar is the server-side registration contract.
type Registrar interface {
	// Register registers the instance with the registry and starts any
	// keepalive/heartbeat needed to keep it alive.
	Register(ctx context.Context, instance *ServiceInstance) error
	// Deregister removes the instance and stops any keepalive.
	Deregister(ctx context.Context, instance *ServiceInstance) error
}

// Watcher streams instance changes for a watched service.
type Watcher interface {
	// Next returns the next snapshot of instances, blocking until one arrives
	// or the watcher is stopped.
	Next() ([]*ServiceInstance, error)
	// Stop terminates the watcher and releases its resources.
	Stop() error
}

// Discovery is the client-side discovery contract.
type Discovery interface {
	// GetService returns the current instances for a service name.
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	// Watch returns a Watcher that streams changes for the service.
	Watch(ctx context.Context, serviceName string) (Watcher, error)
}
