// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package kubernetes implements onexmesh registry.Discovery by discovering
// Pods through Kubernetes EndpointSlices. Registration is a no-op because Pod
// membership is declared via a Service selector and maintained by the
// endpoint controller, following cloud-native practice.
package kubernetes

// ServiceRef maps a onexmesh service name to a concrete Kubernetes
// Service/namespace pair.
type ServiceRef struct {
	// Namespace overrides Options.Namespace for this service.
	Namespace string
	// Service is the Kubernetes Service name (a DNS-1035 label, no dots).
	Service string
	// PortName selects the endpoint port by name; empty inherits
	// Options.PortName.
	PortName string
}

// Options configures the Kubernetes backend.
type Options struct {
	// Namespace is the default namespace for service lookups.
	Namespace string
	// PortName is the default endpoint port name (e.g. "grpc"); empty means
	// the first ready port.
	PortName string
	// Kubeconfig is a path to a kubeconfig file; empty uses in-cluster config.
	Kubeconfig string
	// Services is the explicit mapping from onexmesh service name to a K8s
	// Service/namespace pair. A missing entry yields an error rather than an
	// implicit (and ambiguous) name conversion.
	Services map[string]ServiceRef
}
