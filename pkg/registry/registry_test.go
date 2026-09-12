// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package registry_test

import (
	"testing"

	"github.com/onexstack/onexmesh/pkg/registry"
)

func TestCreateUnknownBackend(t *testing.T) {
	if _, err := registry.CreateRegistrar("unknown", nil); err == nil {
		t.Fatal("expected error for unknown registrar")
	}
	if _, err := registry.CreateDiscovery("unknown", nil); err == nil {
		t.Fatal("expected error for unknown discovery")
	}
}

func TestServiceInstance(t *testing.T) {
	inst := &registry.ServiceInstance{
		ID:        "id-1",
		Name:      "edu.course.student-api",
		Version:   "v0.1.0",
		Endpoints: []string{"grpc://127.0.0.1:9090"},
	}
	if inst.Name != "edu.course.student-api" {
		t.Fatalf("name = %q", inst.Name)
	}
	if len(inst.Endpoints) != 1 {
		t.Fatalf("endpoints = %v", inst.Endpoints)
	}
}
