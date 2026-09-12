// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package selector_test

import (
	"context"
	"errors"
	"testing"

	"github.com/onexstack/onexmesh/pkg/selector"
)

func TestWithCircuitBreakerSelectAndFeedback(t *testing.T) {
	base, err := selector.GetSelector("round_robin")
	if err != nil {
		t.Fatal(err)
	}
	cb := selector.WithCircuitBreaker(base)

	nodes := []selector.Node{
		selector.NewNode("a", "grpc", "svc", 100, nil),
		selector.NewNode("b", "grpc", "svc", 100, nil),
	}

	node, done, err := cb.Select(context.Background(), nodes)
	if err != nil {
		t.Fatalf("Select() err = %v", err)
	}
	if node == nil {
		t.Fatal("Select() returned nil node")
	}

	// Feeding back an error must not panic and should record against the node.
	done(context.Background(), selector.DoneInfo{Err: errors.New("boom")})
}

func TestWithCircuitBreakerNoNodes(t *testing.T) {
	base, err := selector.GetSelector("round_robin")
	if err != nil {
		t.Fatal(err)
	}
	cb := selector.WithCircuitBreaker(base)

	if _, _, err := cb.Select(context.Background(), nil); err == nil {
		t.Fatal("Select() on empty nodes should error")
	}
}
