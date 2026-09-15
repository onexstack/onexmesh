// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"testing"

	"github.com/onexstack/onexmesh/pkg/registry"
)

func TestParseSchemeHost(t *testing.T) {
	scheme, host, err := parseSchemeHost("grpc://127.0.0.1:9090")
	if err != nil || scheme != "grpc" || host != "127.0.0.1:9090" {
		t.Fatalf("parseSchemeHost = (%q, %q, %v)", scheme, host, err)
	}
	// Scheme-less endpoints are preserved (not dropped) so the HTTP client sees
	// the same node set as the rest RoundTripper.
	scheme, host, err = parseSchemeHost("127.0.0.1:9090")
	if err != nil || scheme != "" || host != "127.0.0.1:9090" {
		t.Fatalf("parseSchemeHost(scheme-less) = (%q, %q, %v)", scheme, host, err)
	}
}

func TestMetadataWeight(t *testing.T) {
	if got := metadataWeight(nil); got != 100 {
		t.Fatalf("nil metadata weight = %d, want 100", got)
	}
	if got := metadataWeight(map[string]string{"weight": "42"}); got != 42 {
		t.Fatalf("weight = %d, want 42", got)
	}
	if got := metadataWeight(map[string]string{"weight": "not-a-number"}); got != 100 {
		t.Fatalf("malformed weight = %d, want 100", got)
	}
}

func TestInstancesToNodesSkipsGRPC(t *testing.T) {
	insts := []*registry.ServiceInstance{{
		Name:      "svc",
		Endpoints: []string{"grpc://127.0.0.1:9090", "http://127.0.0.1:8080"},
	}}
	nodes := instancesToNodes("svc", insts)
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1 (grpc endpoint skipped)", len(nodes))
	}
	if nodes[0].Address() != "127.0.0.1:8080" {
		t.Fatalf("address = %q, want http endpoint", nodes[0].Address())
	}
}
