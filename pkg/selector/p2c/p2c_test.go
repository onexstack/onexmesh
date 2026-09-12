// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package p2c

import (
	"context"
	"testing"

	"github.com/onexstack/onexmesh/pkg/selector"
)

func TestSelect(t *testing.T) {
	s, err := selector.GetSelector("p2c")
	if err != nil {
		t.Fatalf("get selector: %v", err)
	}

	nodes := []selector.Node{
		selector.NewNode("a:1", "grpc", "svc", 100, nil),
		selector.NewNode("b:1", "grpc", "svc", 100, nil),
		selector.NewNode("c:1", "grpc", "svc", 100, nil),
	}

	for i := 0; i < 100; i++ {
		n, done, err := s.Select(context.Background(), nodes)
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		if n == nil {
			t.Fatal("selected nil node")
		}
		done(context.Background(), selector.DoneInfo{})
	}
}

func TestSelectEmpty(t *testing.T) {
	s, _ := selector.GetSelector("p2c")
	if _, _, err := s.Select(context.Background(), nil); err == nil {
		t.Fatal("expected error selecting from empty nodes")
	}
}
