// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"reflect"
	"testing"
	"time"
)

func TestResilienceOptionsDialOptions(t *testing.T) {
	o := NewResilienceOptions()
	o.MaxAttempts = 3
	o.BaseBackoff = 100 * time.Millisecond
	o.MaxBackoff = time.Second
	o.BreakerWindow = 10 * time.Second
	o.BreakerProbeInterval = time.Second
	o.Timeout = 5 * time.Second

	opts := o.DialOptions()
	if len(opts) != 3 {
		t.Fatalf("DialOptions() returned %d options, want 3 (retry + breaker + timeout)", len(opts))
	}
}

func TestResilienceOptionsDialOptionsEmpty(t *testing.T) {
	o := &ResilienceOptions{}
	if opts := o.DialOptions(); len(opts) != 0 {
		t.Fatalf("zero resilience config produced %d DialOptions, want 0", len(opts))
	}
}

func TestSelectorOptionsDialOption(t *testing.T) {
	o := NewSelectorOptions()
	o.Strategy = "p2c"
	if o.DialOption() == nil {
		t.Fatal("DialOption() returned nil")
	}
}

func TestServerOptionsServiceInstance(t *testing.T) {
	o := NewServerOptions()
	o.Mesh.ServiceName = "svc"
	o.Mesh.Protocol = "both"
	o.Mesh.GRPCAddr = "127.0.0.1:9090"
	o.Mesh.HTTPAddr = "127.0.0.1:8080"

	inst := o.ServiceInstance()
	if inst.Name != "svc" {
		t.Fatalf("instance name = %q, want %q", inst.Name, "svc")
	}
	want := []string{"grpc://127.0.0.1:9090", "http://127.0.0.1:8080"}
	if !reflect.DeepEqual(inst.Endpoints, want) {
		t.Fatalf("instance endpoints = %v, want %v", inst.Endpoints, want)
	}
}

func TestServerOptionsServiceInstanceFor(t *testing.T) {
	o := NewServerOptions()
	o.Mesh.ServiceName = "svc"
	o.Mesh.Protocol = "both"
	o.Mesh.GRPCAddr = "127.0.0.1:9090"
	o.Mesh.HTTPAddr = "127.0.0.1:8080"

	grpcInst := o.ServiceInstanceFor("grpc")
	if grpcInst == nil {
		t.Fatal("ServiceInstanceFor(grpc) = nil")
	}
	if want := []string{"grpc://127.0.0.1:9090"}; !reflect.DeepEqual(grpcInst.Endpoints, want) {
		t.Fatalf("grpc endpoints = %v, want %v", grpcInst.Endpoints, want)
	}

	httpInst := o.ServiceInstanceFor("http")
	if httpInst == nil {
		t.Fatal("ServiceInstanceFor(http) = nil")
	}
	if want := []string{"http://127.0.0.1:8080"}; !reflect.DeepEqual(httpInst.Endpoints, want) {
		t.Fatalf("http endpoints = %v, want %v", httpInst.Endpoints, want)
	}
}

func TestServerOptionsServiceInstanceForExcluded(t *testing.T) {
	o := NewServerOptions()
	o.Mesh.ServiceName = "svc"
	o.Mesh.Protocol = "grpc"
	o.Mesh.GRPCAddr = "127.0.0.1:9090"

	if inst := o.ServiceInstanceFor("http"); inst != nil {
		t.Fatalf("ServiceInstanceFor(http) = %v, want nil for grpc-only", inst)
	}
}

func TestServerOptionsBuildClientDialOptionsNone(t *testing.T) {
	o := NewServerOptions()
	o.Registry.Type = "none"

	if _, err := o.BuildClientDialOptions(); err == nil {
		t.Fatal("BuildClientDialOptions() with registry type none should error")
	}
}

func TestRegistryOptionsNewRegistrarNone(t *testing.T) {
	o := NewRegistryOptions()
	o.Type = "none"

	r, err := o.NewRegistrar("127.0.0.1", 9090, "grpc")
	if err != nil {
		t.Fatalf("NewRegistrar(none) returned error: %v", err)
	}
	if r != nil {
		t.Fatal("NewRegistrar(none) should return a nil registrar")
	}
}
