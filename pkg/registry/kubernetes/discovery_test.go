// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kubernetes

import (
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
)

func ptr[T any](v T) *T { return &v }

func TestSelectPort(t *testing.T) {
	ports := []discoveryv1.EndpointPort{
		{Name: ptr("grpc"), Port: ptr(int32(9090))},
		{Name: ptr("http"), Port: ptr(int32(8080))},
	}

	tests := []struct {
		name     string
		ports    []discoveryv1.EndpointPort
		portName string
		want     int32
		ok       bool
	}{
		{name: "empty", ports: nil, portName: "grpc", want: 0, ok: false},
		{name: "first when empty name", ports: ports, portName: "", want: 9090, ok: true},
		{name: "match by name", ports: ports, portName: "http", want: 8080, ok: true},
		{name: "no match", ports: ports, portName: "metrics", want: 0, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := selectPort(tt.ports, tt.portName)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("selectPort() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestEndpointSlicesToInstances(t *testing.T) {
	ready := ptr(true)
	notReady := ptr(false)

	slices := []discoveryv1.EndpointSlice{
		{
			Ports: []discoveryv1.EndpointPort{{Name: ptr("grpc"), Port: ptr(int32(9090))}},
			Endpoints: []discoveryv1.Endpoint{
				{Conditions: discoveryv1.EndpointConditions{Ready: ready}, Addresses: []string{"10.0.0.1", "10.0.0.2"}},
				{Conditions: discoveryv1.EndpointConditions{Ready: notReady}, Addresses: []string{"10.0.0.3"}},
			},
		},
	}

	instances, err := endpointSlicesToInstances("svc", slices, "grpc")
	if err != nil {
		t.Fatalf("endpointSlicesToInstances() error = %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("len(instances) = %d, want 2", len(instances))
	}
	if instances[0].Endpoints[0] != "grpc://10.0.0.1:9090" {
		t.Fatalf("endpoint = %q", instances[0].Endpoints[0])
	}
}

func TestEndpointSlicesToInstancesMissingPort(t *testing.T) {
	ready := ptr(true)
	slices := []discoveryv1.EndpointSlice{
		{
			Ports: []discoveryv1.EndpointPort{{Name: ptr("http"), Port: ptr(int32(8080))}},
			Endpoints: []discoveryv1.Endpoint{
				{Conditions: discoveryv1.EndpointConditions{Ready: ready}, Addresses: []string{"10.0.0.1"}},
			},
		},
	}
	if _, err := endpointSlicesToInstances("svc", slices, "grpc"); err == nil {
		t.Fatal("expected error for missing port")
	}
}

func TestResolve(t *testing.T) {
	d := &discovery{opts: Options{
		Namespace: "ns",
		PortName:  "grpc",
		Services:  map[string]ServiceRef{"svc": {Service: "backend"}},
	}}

	t.Run("missing mapping", func(t *testing.T) {
		if _, err := d.resolve("unknown"); err == nil {
			t.Fatal("expected error for missing mapping")
		}
	})

	t.Run("inherits defaults", func(t *testing.T) {
		ref, err := d.resolve("svc")
		if err != nil {
			t.Fatalf("resolve() error = %v", err)
		}
		if ref.Namespace != "ns" {
			t.Fatalf("namespace = %q, want ns", ref.Namespace)
		}
		if ref.PortName != "grpc" {
			t.Fatalf("portName = %q, want grpc", ref.PortName)
		}
		if ref.Service != "backend" {
			t.Fatalf("service = %q, want backend", ref.Service)
		}
	})
}
