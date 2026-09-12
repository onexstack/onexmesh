// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package app_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"github.com/onexstack/onexmesh/pkg/app"
	"github.com/onexstack/onexmesh/pkg/options"
)

// freeAddr reserves an ephemeral port and returns its address.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen ephemeral: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// TestRunMeshStartAndStop verifies the composition root starts the HTTP+gRPC
// servers, serves a request, and gracefully stops on context cancellation.
func TestRunMeshStartAndStop(t *testing.T) {
	grpcAddr := freeAddr(t)
	httpAddr := freeAddr(t)

	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "test"
	opts.Mesh.Protocol = "both"
	opts.Mesh.GRPCAddr = grpcAddr
	opts.Mesh.HTTPAddr = httpAddr
	opts.Registry.Type = "none"

	engine := gin.New()
	engine.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	run := app.RunMesh(opts, func(s grpc.ServiceRegistrar) {}, engine)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	// Poll the HTTP endpoint until the server is up.
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get("http://" + httpAddr + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("http server did not come up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunMesh returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunMesh did not return after cancellation")
	}
}
