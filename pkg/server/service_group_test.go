// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/onexstack/onexmesh/pkg/server"
)

// fakeServer records lifecycle calls in shared order.
type fakeServer struct {
	mu    *sync.Mutex
	order *[]string
	name  string
	block chan struct{}
	once  sync.Once
}

func (f *fakeServer) Start(ctx context.Context) error {
	f.mu.Lock()
	*f.order = append(*f.order, "start:"+f.name)
	f.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil
	case <-f.block:
		return nil
	}
}

func (f *fakeServer) Stop(ctx context.Context) error {
	f.mu.Lock()
	*f.order = append(*f.order, "stop:"+f.name)
	f.mu.Unlock()
	return nil
}

// failingServer always fails Stop, used to assert error aggregation.
type failingServer struct {
	name  string
	mu    *sync.Mutex
	stops *int
}

func (f *failingServer) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (f *failingServer) Stop(ctx context.Context) error {
	f.mu.Lock()
	*f.stops++
	f.mu.Unlock()
	return errors.New("stop failed")
}

// TestServiceGroupReverseStop verifies servers stop in reverse start order.
func TestServiceGroupReverseStop(t *testing.T) {
	var mu sync.Mutex
	var order []string
	block := make(chan struct{})

	s1 := &fakeServer{mu: &mu, order: &order, name: "a", block: block}
	s2 := &fakeServer{mu: &mu, order: &order, name: "b", block: block}

	g := server.NewServiceGroup()
	g.Add("a", s1)
	g.Add("b", s2)

	ctx, cancel := context.WithCancel(context.Background())

	startDone := make(chan error, 1)
	go func() { startDone <- g.Start(ctx) }()

	// Wait until both have started.
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(order)
		mu.Unlock()
		if n >= 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Stop the group.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	if err := g.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	stopCancel()

	// Signal the start goroutines to unblock.
	close(block)
	<-startDone
	cancel()

	mu.Lock()
	defer mu.Unlock()
	// Expect stop in reverse order: b then a.
	want := []string{"stop:b", "stop:a"}
	if len(order) < 2 {
		t.Fatalf("order too short: %v", order)
	}
	stopOrder := order[len(order)-2:]
	for i := range want {
		if stopOrder[i] != want[i] {
			t.Fatalf("stop order = %v, want %v", stopOrder, want)
		}
	}
}

// TestServiceGroupStopAggregatesErrors verifies a failing stop does not skip the
// remaining servers and that all errors are reported.
func TestServiceGroupStopAggregatesErrors(t *testing.T) {
	var mu sync.Mutex
	var stops int

	g := server.NewServiceGroup()
	g.Add("a", &failingServer{name: "a", mu: &mu, stops: &stops})
	g.Add("b", &failingServer{name: "b", mu: &mu, stops: &stops})

	err := g.Stop(context.Background())
	if err == nil {
		t.Fatal("Stop() = nil, want aggregated errors")
	}

	mu.Lock()
	got := stops
	mu.Unlock()
	if got != 2 {
		t.Fatalf("stops = %d, want 2 (both servers must be stopped)", got)
	}
}
