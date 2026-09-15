// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proto "github.com/onexstack/onexmesh/examples/helloworld/proto"
	"github.com/onexstack/onexmesh/pkg/middleware"
	"github.com/onexstack/onexmesh/pkg/options"
	"github.com/onexstack/onexmesh/pkg/server"
	"github.com/onexstack/onexmesh/pkg/transport"
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

// TestMeshServerStartAndStop verifies the composition root starts the HTTP+gRPC
// servers, serves a request, and gracefully stops on context cancellation.
func TestMeshServerStartAndStop(t *testing.T) {
	grpcAddr := freeAddr(t)
	httpAddr := freeAddr(t)

	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "test"
	opts.Mesh.Protocol = "both"
	opts.Mesh.GRPCAddr = grpcAddr
	opts.Mesh.HTTPAddr = httpAddr
	opts.Registry.Type = "none"

	engine, err := server.NewGinEngine(opts)
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	mesh := server.NewMeshServer(
		opts,
		server.WithGRPCRegister(func(s grpc.ServiceRegistrar) {}),
		server.WithGinEngine(engine),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mesh.Run(ctx) }()

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
			t.Fatalf("MeshServer.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MeshServer.Run did not return after cancellation")
	}
}

// TestMeshServerWithServices verifies that a proto-first Method serves HTTP (body
// binding) and the RouteGroups extension point registers HTTP-only routes.
func TestMeshServerWithServices(t *testing.T) {
	grpcAddr := freeAddr(t)
	httpAddr := freeAddr(t)

	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "helloworld.Greeter"
	opts.Mesh.Protocol = "both"
	opts.Mesh.GRPCAddr = grpcAddr
	opts.Mesh.HTTPAddr = httpAddr
	opts.Registry.Type = "none"

	svc := server.NewService(
		"helloworld.Greeter",
		server.NewMethod(
			"SayHello", "POST", "/hello", "*",
			func() *proto.HelloRequest { return &proto.HelloRequest{} },
			func(_ context.Context, r *proto.HelloRequest) (*proto.HelloReply, error) {
				return &proto.HelloReply{Message: "Hello " + r.GetName()}, nil
			},
		),
	)
	svc.RouteGroups = []*server.RouteGroup{
		server.NewGroup("").GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") }),
	}

	mesh := server.NewMeshServer(opts, server.WithService(svc))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mesh.Run(ctx) }()

	// Poll the body endpoint until the server is up, then POST a body.
	deadline := time.Now().Add(5 * time.Second)
	var resp *http.Response
	for {
		r, err := http.Post("http://"+httpAddr+"/hello", "application/json",
			strings.NewReader(`{"name":"world"}`))
		if err == nil {
			resp = r
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("http server did not come up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /hello status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "Hello world") {
		t.Fatalf("POST /hello body = %q, want to contain %q", body, "Hello world")
	}

	// Verify the RouteGroups extension route.
	pr, err := http.Get("http://" + httpAddr + "/ping")
	if err != nil {
		cancel()
		t.Fatalf("GET /ping: %v", err)
	}
	if pr.StatusCode != http.StatusOK {
		t.Fatalf("GET /ping status = %d, want 200", pr.StatusCode)
	}
	_ = pr.Body.Close()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("MeshServer.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MeshServer.Run did not return after cancellation")
	}
}

// TestMeshServerWithEngine verifies the native-gin path: an engine built via
// server.NewGinEngine carries native gin routes alongside a proto-first service
// registered through WithGinEngine + WithService.
func TestMeshServerWithEngine(t *testing.T) {
	grpcAddr := freeAddr(t)
	httpAddr := freeAddr(t)

	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "helloworld.Greeter"
	opts.Mesh.Protocol = "both"
	opts.Mesh.GRPCAddr = grpcAddr
	opts.Mesh.HTTPAddr = httpAddr
	opts.Registry.Type = "none"

	engine, err := server.NewGinEngine(opts)
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/native", func(c *gin.Context) {
		c.String(http.StatusOK, "native")
	})

	srv := &greeterImpl{}
	svc := proto.NewGreeterService(srv)

	mesh := server.NewMeshServer(opts, server.WithGinEngine(engine), server.WithService(svc))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mesh.Run(ctx) }()

	// Poll the native gin route until the server is up.
	deadline := time.Now().Add(5 * time.Second)
	var resp *http.Response
	for {
		r, err := http.Get("http://" + httpAddr + "/native")
		if err == nil {
			resp = r
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("http server did not come up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /native status = %d, want 200", resp.StatusCode)
	}

	// Verify the proto-first route is served by the same engine.
	pr, err := http.Get("http://" + httpAddr + "/helloworld/world")
	if err != nil {
		cancel()
		t.Fatalf("GET /helloworld/world: %v", err)
	}
	body, _ := io.ReadAll(pr.Body)
	_ = pr.Body.Close()
	if pr.StatusCode != http.StatusOK {
		t.Fatalf("GET /helloworld/world status = %d, want 200", pr.StatusCode)
	}
	if !strings.Contains(string(body), "Hello world") {
		t.Fatalf("GET /helloworld/world body = %q, want to contain %q", body, "Hello world")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("MeshServer.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MeshServer.Run did not return after cancellation")
	}
}

// TestMeshServerRegistered verifies the plugin-style HTTP route registration: a
// route module self-registered via server.RegisterHTTPRoute is auto-discovered
// and served by WithRoute(AllHTTPRoutes()) without being passed explicitly.
func TestMeshServerRegistered(t *testing.T) {
	httpAddr := freeAddr(t)

	// Simulate a business package self-registering its HTTP route in init().
	server.RegisterHTTPRoute(
		"registered-healthz",
		server.NewGroup("").GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") }),
	)

	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "test"
	opts.Mesh.Protocol = "http"
	opts.Mesh.HTTPAddr = httpAddr
	opts.Registry.Type = "none"

	mesh := server.NewMeshServer(opts, server.WithRoute(server.AllHTTPRoutes()...))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mesh.Run(ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	var resp *http.Response
	for {
		r, err := http.Get("http://" + httpAddr + "/healthz")
		if err == nil {
			resp = r
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("http server did not come up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("MeshServer.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MeshServer.Run did not return after cancellation")
	}
}

// greeterImpl is a local implementation of proto.GreeterServer used to verify the
// protoc-gen-onexmesh generated HTTP routes (path + body) call the same handler.
type greeterImpl struct {
	proto.UnimplementedGreeterServer
}

func (g *greeterImpl) SayHello(_ context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
	return &proto.HelloReply{Message: "Hello " + req.GetName()}, nil
}

func (g *greeterImpl) SayHelloPost(_ context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
	return &proto.HelloReply{Message: "Hello (post) " + req.GetName()}, nil
}

// TestNewEngineAppliesMiddleware verifies the timing fix: a route-level
// middleware bound via MiddlewareRoutes is injected by NewGinEngine before any
// route is registered, so it actually runs on the route (and writes the header).
func TestNewEngineAppliesMiddleware(t *testing.T) {
	httpAddr := freeAddr(t)

	middleware.Register("test-header", func() middleware.Middleware {
		return func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req any) (any, error) {
				if tr, ok := transport.FromServerContext(ctx); ok {
					tr.ReplyHeader().Set("X-Test", "ok")
				}
				return next(ctx, req)
			}
		}
	})

	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "test"
	opts.Mesh.Protocol = "http"
	opts.Mesh.HTTPAddr = httpAddr
	opts.Registry.Type = "none"
	opts.Mesh.MiddlewareRoutes = []string{"*=test-header"}

	engine, err := server.NewGinEngine(opts)
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	mesh := server.NewMeshServer(opts, server.WithGinEngine(engine))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mesh.Run(ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	var resp *http.Response
	for {
		r, err := http.Get("http://" + httpAddr + "/healthz")
		if err == nil {
			resp = r
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("http server did not come up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if got := resp.Header.Get("X-Test"); got != "ok" {
		t.Fatalf("X-Test header = %q, want %q (middleware not applied)", got, "ok")
	}
	_ = resp.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("MeshServer.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MeshServer.Run did not return after cancellation")
	}
}

// TestMeshServerRejectsRawEngine verifies the guard: a raw gin.New() engine passed
// to MeshServer is rejected instead of silently bypassing the middleware chain.
func TestMeshServerRejectsRawEngine(t *testing.T) {
	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "test"
	opts.Mesh.Protocol = "http"
	opts.Mesh.HTTPAddr = freeAddr(t)
	opts.Registry.Type = "none"

	mesh := server.NewMeshServer(opts, server.WithGinEngine(gin.New()))
	if err := mesh.Run(context.Background()); err == nil {
		t.Fatal("expected error for raw gin.New() engine, got nil")
	}
}

// TestMeshServerGRPC verifies the gRPC-only entry point serves a registered
// service without a gin engine.
func TestMeshServerGRPC(t *testing.T) {
	grpcAddr := freeAddr(t)

	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "helloworld.Greeter"
	opts.Mesh.Protocol = "grpc"
	opts.Mesh.GRPCAddr = grpcAddr
	opts.Registry.Type = "none"

	srv := &greeterImpl{}
	mesh := server.NewMeshServer(opts, server.WithGRPCRegister(func(s grpc.ServiceRegistrar) {
		proto.RegisterGreeterServer(s, srv)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mesh.Run(ctx) }()

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cancel()
		t.Fatalf("grpc new client: %v", err)
	}
	defer conn.Close()
	client := proto.NewGreeterClient(conn)

	var resp *proto.HelloReply
	deadline := time.Now().Add(5 * time.Second)
	for {
		r, err := client.SayHello(ctx, &proto.HelloRequest{Name: "world"})
		if err == nil {
			resp = r
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("grpc SayHello: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if resp.GetMessage() != "Hello world" {
		t.Fatalf("message = %q, want %q", resp.GetMessage(), "Hello world")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("MeshServer.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MeshServer.Run did not return after cancellation")
	}
}

// TestMeshServerHTTP verifies the HTTP-only entry point serves gin routes without
// a gRPC server.
func TestMeshServerHTTP(t *testing.T) {
	httpAddr := freeAddr(t)

	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "test"
	opts.Mesh.Protocol = "http"
	opts.Mesh.HTTPAddr = httpAddr
	opts.Registry.Type = "none"

	engine, err := server.NewGinEngine(opts)
	if err != nil {
		t.Fatalf("NewGinEngine: %v", err)
	}
	engine.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	mesh := server.NewMeshServer(opts, server.WithGinEngine(engine))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mesh.Run(ctx) }()

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
			t.Fatalf("MeshServer.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MeshServer.Run did not return after cancellation")
	}
}

// TestGeneratedHTTPRoute verifies that protobuf-IDL-driven HTTP routes generated
// by protoc-gen-onexmesh are served: GET /helloworld/{name} binds the path
// parameter and POST /helloworld binds the body, both calling the same
// proto.GreeterServer implementation used by gRPC.
func TestGeneratedHTTPRoute(t *testing.T) {
	grpcAddr := freeAddr(t)
	httpAddr := freeAddr(t)

	opts := options.NewServerOptions()
	opts.Mesh.ServiceName = "helloworld.Greeter"
	opts.Mesh.Protocol = "both"
	opts.Mesh.GRPCAddr = grpcAddr
	opts.Mesh.HTTPAddr = httpAddr
	opts.Registry.Type = "none"

	srv := &greeterImpl{}
	svc := proto.NewGreeterService(srv)

	mesh := server.NewMeshServer(opts, server.WithService(svc))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mesh.Run(ctx) }()

	// Wait for the server to come up, then exercise both generated routes.
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get("http://" + httpAddr + "/helloworld/world")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /helloworld/world status = %d", resp.StatusCode)
			}
			if !strings.Contains(string(body), "Hello world") {
				t.Fatalf("GET body = %q, want to contain %q", body, "Hello world")
			}
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("http server did not come up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// POST /helloworld with a JSON body bound to the whole request message.
	resp, err := http.Post("http://"+httpAddr+"/helloworld", "application/json",
		strings.NewReader(`{"name":"onex"}`))
	if err != nil {
		cancel()
		t.Fatalf("POST /helloworld: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /helloworld status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Hello (post) onex") {
		t.Fatalf("POST body = %q, want to contain %q", body, "Hello (post) onex")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("MeshServer.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MeshServer.Run did not return after cancellation")
	}
}
