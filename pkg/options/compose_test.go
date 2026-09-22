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

// meshOptionsForTest returns options whose advertised address is fixed, so the
// endpoint assertions below are deterministic rather than dependent on the
// machine's interfaces.
func meshOptionsForTest(t *testing.T, protocol string) *ServerOptions {
	t.Helper()
	o := NewServerOptions()
	o.Mesh.ServiceName = "svc"
	o.Mesh.Protocol = protocol
	o.Mesh.GRPCAddr = "127.0.0.1:9090"
	o.Mesh.HTTPAddr = "127.0.0.1:8080"
	o.Mesh.Host = "127.0.0.1"
	return o
}

func TestServerOptionsServiceInstance(t *testing.T) {
	o := meshOptionsForTest(t, "both")

	inst, err := o.ServiceInstance()
	if err != nil {
		t.Fatalf("ServiceInstance() error = %v", err)
	}
	if inst.Name != "svc" {
		t.Fatalf("instance name = %q, want %q", inst.Name, "svc")
	}
	want := []string{"grpc://127.0.0.1:9090", "http://127.0.0.1:8080"}
	if !reflect.DeepEqual(inst.Endpoints, want) {
		t.Fatalf("instance endpoints = %v, want %v", inst.Endpoints, want)
	}
}

func TestServerOptionsServiceInstanceFor(t *testing.T) {
	o := meshOptionsForTest(t, "both")

	grpcInst, err := o.ServiceInstanceFor("grpc")
	if err != nil {
		t.Fatalf("ServiceInstanceFor(grpc) error = %v", err)
	}
	if grpcInst == nil {
		t.Fatal("ServiceInstanceFor(grpc) = nil")
	}
	if want := []string{"grpc://127.0.0.1:9090"}; !reflect.DeepEqual(grpcInst.Endpoints, want) {
		t.Fatalf("grpc endpoints = %v, want %v", grpcInst.Endpoints, want)
	}

	httpInst, err := o.ServiceInstanceFor("http")
	if err != nil {
		t.Fatalf("ServiceInstanceFor(http) error = %v", err)
	}
	if httpInst == nil {
		t.Fatal("ServiceInstanceFor(http) = nil")
	}
	if want := []string{"http://127.0.0.1:8080"}; !reflect.DeepEqual(httpInst.Endpoints, want) {
		t.Fatalf("http endpoints = %v, want %v", httpInst.Endpoints, want)
	}
}

func TestServerOptionsServiceInstanceForExcluded(t *testing.T) {
	o := meshOptionsForTest(t, "grpc")

	inst, err := o.ServiceInstanceFor("http")
	if err != nil {
		t.Fatalf("ServiceInstanceFor(http) error = %v", err)
	}
	if inst != nil {
		t.Fatalf("ServiceInstanceFor(http) = %v, want nil for grpc-only", inst)
	}
}

// TestServerOptionsServiceInstanceAdvertisesHost pins the endpoint half of the
// listen-address bug: a backend that registers the instance itself (etcd,
// kubernetes) publishes inst.Endpoints, which used to be built from the listen
// address — "grpc://0.0.0.0:9090", an entry that resolves to whoever reads it.
func TestServerOptionsServiceInstanceAdvertisesHost(t *testing.T) {
	o := NewServerOptions()
	o.Mesh.ServiceName = "svc"
	o.Mesh.Protocol = "grpc"
	// Listening on every interface, but reachable at one address.
	o.Mesh.GRPCAddr = "0.0.0.0:9090"
	o.Mesh.Host = "10.0.0.7"

	inst, err := o.ServiceInstanceFor("grpc")
	if err != nil {
		t.Fatalf("ServiceInstanceFor(grpc) error = %v", err)
	}
	if want := []string{"grpc://10.0.0.7:9090"}; !reflect.DeepEqual(inst.Endpoints, want) {
		t.Fatalf("endpoints = %v, want %v (the advertised host, not the listen address)", inst.Endpoints, want)
	}
}

// TestServerOptionsServiceInstanceCarriesEnv pins that the environment reaches
// the instance metadata, which is what lets one registry namespace hold several
// environments without a caller mistaking a developer's laptop for a replica.
func TestServerOptionsServiceInstanceCarriesEnv(t *testing.T) {
	o := meshOptionsForTest(t, "grpc")
	o.Mesh.Env = "prod"
	o.Mesh.Version = "1.4.2"
	o.Mesh.Metadata = map[string]string{"zone": "sh"}

	inst, err := o.ServiceInstanceFor("grpc")
	if err != nil {
		t.Fatalf("ServiceInstanceFor(grpc) error = %v", err)
	}
	if inst.Version != "1.4.2" {
		t.Errorf("version = %q, want 1.4.2", inst.Version)
	}
	if got := inst.Metadata[MetadataKeyEnv]; got != "prod" {
		t.Errorf("metadata[%q] = %q, want prod", MetadataKeyEnv, got)
	}
	if got := inst.Metadata["zone"]; got != "sh" {
		t.Errorf("metadata[zone] = %q, want sh", got)
	}
}

// TestServerOptionsServiceInstanceNoMetadataWhenUnset pins that a service with
// nothing to say writes no metadata at all, rather than an empty map into every
// backend that serializes the instance.
func TestServerOptionsServiceInstanceNoMetadataWhenUnset(t *testing.T) {
	o := meshOptionsForTest(t, "grpc")

	inst, err := o.ServiceInstanceFor("grpc")
	if err != nil {
		t.Fatalf("ServiceInstanceFor(grpc) error = %v", err)
	}
	if inst.Metadata != nil {
		t.Errorf("metadata = %v, want nil when nothing is configured", inst.Metadata)
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
