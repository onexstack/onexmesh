// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package selector_test

import (
	"context"
	"testing"

	"github.com/onexstack/onexmesh/pkg/selector"
	_ "github.com/onexstack/onexmesh/pkg/selector/random"
	_ "github.com/onexstack/onexmesh/pkg/selector/roundrobin"
	_ "github.com/onexstack/onexmesh/pkg/selector/weighted"
)

func nodes() []selector.Node {
	return []selector.Node{
		selector.NewNode("127.0.0.1:1", "grpc", "svc", 100, nil),
		selector.NewNode("127.0.0.1:2", "grpc", "svc", 100, nil),
		selector.NewNode("127.0.0.1:3", "grpc", "svc", 100, nil),
	}
}

func TestGetSelector(t *testing.T) {
	for _, name := range []string{"round_robin", "random", "weighted"} {
		s, err := selector.GetSelector(name)
		if err != nil {
			t.Fatalf("GetSelector(%q): %v", name, err)
		}
		if s == nil {
			t.Fatalf("GetSelector(%q) returned nil", name)
		}
	}
	if _, err := selector.GetSelector("unknown"); err == nil {
		t.Fatal("expected error for unknown selector")
	}
}

func TestRoundRobinCycles(t *testing.T) {
	s, _ := selector.GetSelector("round_robin")
	seen := map[string]bool{}
	for i := 0; i < len(nodes()); i++ {
		n, _, err := s.Select(context.Background(), nodes())
		if err != nil {
			t.Fatal(err)
		}
		seen[n.Address()] = true
	}
	if len(seen) != len(nodes()) {
		t.Fatalf("round robin visited %d distinct nodes, want %d", len(seen), len(nodes()))
	}
}

func TestEmptyNodes(t *testing.T) {
	for _, name := range []string{"round_robin", "random", "weighted"} {
		s, _ := selector.GetSelector(name)
		if _, _, err := s.Select(context.Background(), nil); err == nil {
			t.Fatalf("expected error for empty nodes with %s", name)
		}
	}
}
