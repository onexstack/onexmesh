// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	helloworld "github.com/onexstack/onexmesh/examples/helloworld/proto"
	"github.com/onexstack/onexmesh/pkg/client"
	"github.com/onexstack/onexmesh/pkg/registry"
)

// staticDiscovery is an in-memory Discovery returning a fixed instance set.
// It proves the full resolve -> dial -> call flow without an external backend.
type staticDiscovery struct {
	mu        sync.RWMutex
	instances []*registry.ServiceInstance
}

func (d *staticDiscovery) GetService(ctx context.Context, name string) ([]*registry.ServiceInstance, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.instances, nil
}

func (d *staticDiscovery) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	return &staticWatcher{}, nil
}

type staticWatcher struct{}

func (w *staticWatcher) Next() ([]*registry.ServiceInstance, error) {
	// Block forever; the resolver uses GetService for the initial snapshot and
	// this test does not exercise incremental updates.
	select {}
}

func (w *staticWatcher) Stop() error { return nil }

// greeter implements the helloworld Greeter server.
type greeter struct {
	helloworld.UnimplementedGreeterServer
}

func (g *greeter) SayHello(ctx context.Context, req *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	return &helloworld.HelloReply{Message: "hello " + req.GetName()}, nil
}

func init() {
	// Register a "static" discovery backend whose options are the Discovery
	// itself, so tests can inject in-memory backends.
	registry.RegisterDiscovery("static", func(opts any) (registry.Discovery, error) {
		d, ok := opts.(registry.Discovery)
		if !ok {
			return nil, fmt.Errorf("static: expected registry.Discovery, got %T", opts)
		}
		return d, nil
	})
}

func TestDialDiscoversAndCalls(t *testing.T) {
	// Start an in-process gRPC server on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	helloworld.RegisterGreeterServer(srv, &greeter{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	// A static discovery returning the live server's address.
	discovery := &staticDiscovery{instances: []*registry.ServiceInstance{{
		ID:        "i1",
		Name:      "edu.course.student-api",
		Endpoints: []string{"grpc://" + lis.Addr().String()},
	}}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := client.Dial(ctx, "edu.course.student-api", client.WithRegistry("static", discovery))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	grpcClient := helloworld.NewGreeterClient(conn)
	resp, err := grpcClient.SayHello(ctx, &helloworld.HelloRequest{Name: "world"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetMessage() != "hello world" {
		t.Fatalf("message = %q, want %q", resp.GetMessage(), "hello world")
	}
}

func TestDialMissingRegistry(t *testing.T) {
	_, err := client.Dial(context.Background(), "svc")
	if err == nil {
		t.Fatal("expected error when registry is not configured")
	}
}

func TestDialUnknownRegistry(t *testing.T) {
	_, err := client.Dial(context.Background(), "svc", client.WithRegistry("no-such", nil))
	if err == nil {
		t.Fatal("expected error for unknown registry")
	}
}
