// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"github.com/onexstack/onexmesh/pkg/middleware"
	"github.com/onexstack/onexmesh/pkg/middleware/matcher"
	"github.com/onexstack/onexmesh/pkg/options"
	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/server"
)

// meshShutdownTimeout is the budget for graceful shutdown and deregistration.
const meshShutdownTimeout = 10 * time.Second

// RunMesh returns a RunFunc that assembles a dual-protocol service from opts and
// wires it into the OneX app lifecycle. register registers the gRPC business
// services (may be nil for HTTP-only); engine is the gin engine serving HTTP
// (may be nil for gRPC-only). On start the service is registered to the registry
// with the standard middleware chain; on context cancellation it is gracefully
// stopped and deregistered.
//
// The registry type comes from opts.Registry.Type; "none" skips registration.
// This is the composition root that turns ServerOptions flags into a running
// service, so generated SDKs and examples no longer wire dependencies by hand.
func RunMesh(opts *options.ServerOptions, register func(grpc.ServiceRegistrar), engine *gin.Engine) RunFunc {
	return func(ctx context.Context) error {
		// Resolve the middleware chain once — route-aware via the matcher when
		// configured, otherwise the flat global chain — then adapt it to both the
		// unary and stream gRPC bridges and the gin bridge from the same source.
		var mws []middleware.Middleware
		if m, err := opts.BuildMatcher(); err != nil {
			return err
		} else if m != nil {
			mws = []middleware.Middleware{matcher.Match(m)}
		} else {
			mws = opts.BuildMiddleware()
		}

		var unaryInts []grpc.UnaryServerInterceptor
		var streamInts []grpc.StreamServerInterceptor
		for _, mw := range mws {
			unaryInts = append(unaryInts, middleware.UnaryServerInterceptor(mw))
			streamInts = append(streamInts, middleware.StreamServerInterceptor(mw))
		}

		group := server.NewServiceGroup()

		if register != nil && options.ProtocolUsesGRPC(opts.Mesh.Protocol) {
			grpcReg, err := buildRegistrarFor(opts, "grpc", opts.Mesh.GRPCAddr)
			if err != nil {
				return err
			}
			grpcSrv := server.NewGRPCServer(opts.Mesh.GRPCAddr, register,
				grpc.ChainUnaryInterceptor(unaryInts...),
				grpc.ChainStreamInterceptor(streamInts...),
			)
			if grpcReg != nil {
				grpcSrv.WithRegistrar(grpcReg, opts.ServiceInstanceFor("grpc"))
			}
			group.Add("grpc", grpcSrv)
		}

		if engine != nil && options.ProtocolUsesHTTP(opts.Mesh.Protocol) {
			for _, m := range mws {
				engine.Use(middleware.GinHandler(m))
			}
			httpReg, err := buildRegistrarFor(opts, "http", opts.Mesh.HTTPAddr)
			if err != nil {
				return err
			}
			var httpOpts []server.HTTPOption
			if httpReg != nil {
				httpOpts = append(httpOpts, server.WithRegistrar(httpReg, opts.ServiceInstanceFor("http")))
			}
			group.Add("http", server.NewHTTPServer(opts.Mesh.HTTPAddr, engine, httpOpts...))
		}

		startErr := group.Start(ctx)

		// Always deregister and gracefully stop remaining servers, whether Start
		// returned due to cancellation or a sibling failure.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), meshShutdownTimeout)
		defer cancel()
		if err := group.Stop(shutdownCtx); err != nil {
			return fmt.Errorf("stop service group: %w", err)
		}

		return startErr
	}
}

// buildRegistrarFor resolves the local registration host/port from the listen
// address matching the given protocol and creates the registrar selected by the
// registry options. It returns (nil, nil) when the registry type is "none".
func buildRegistrarFor(opts *options.ServerOptions, protocol, addr string) (registry.Registrar, error) {
	host, port := splitHostPort(addr)
	registrar, err := opts.Registry.NewRegistrar(host, port, protocol)
	if err != nil {
		return nil, fmt.Errorf("build registrar: %w", err)
	}
	return registrar, nil
}

// splitHostPort splits a listen "host:port" address into its components,
// returning ("", 0) on malformed input. It uses net.SplitHostPort so it handles
// bracketed IPv6 literals, which the Nacos backend's local splitHostPort (a
// LastIndex split for server addresses) intentionally does not.
func splitHostPort(addr string) (string, int) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 0
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
}

// MeshServer adapts RunMesh into a blocking Run / GracefulStop server, mirroring
// the onexstack pkg/server.Server contract so generated code can treat onexmesh
// exactly like the other web frameworks (Run in a goroutine, GracefulStop on
// shutdown). RunMesh already performs graceful stop and deregistration inside
// Run, so GracefulStop is an idempotent no-op kept purely for interface
// symmetry.
type MeshServer struct {
	run RunFunc
}

// RunMeshWithServices is the recommended composition root for dual-protocol
// services. Each Service.Methods is auto-registered as gRPC unary methods (via
// Service.RegisterGRPC) and as HTTP routes (via middleware.HTTPHandler), so a
// single strongly-typed handler serves both protocols. Service.Register and
// Service.RegisterHTTP are the symmetric extension points for gRPC streaming and
// IDL-generated HTTP routes respectively. It delegates to RunMesh, so
// middleware, registration and lifecycle behavior are identical.
func RunMeshWithServices(opts *options.ServerOptions, services ...server.Service) RunFunc {
	return RunMesh(opts, registerServices(services), buildEngine(services, nil))
}

// RunMeshWithRoutes assembles a pure HTTP service from explicit HTTPRoute
// plugins (no gRPC methods). Each route registers itself onto the gin engine;
// RunMesh wires the standard middleware chain, registration and lifecycle.
func RunMeshWithRoutes(opts *options.ServerOptions, routes ...server.HTTPRoute) RunFunc {
	return RunMesh(opts, nil, buildEngine(nil, routes))
}

// RunMeshRegistered auto-discovers every HTTPRoute registered via
// server.RegisterHTTPRoute and assembles them alongside any explicit services.
// Business packages self-register in init(), so the composition root needs no
// explicit import of each route module.
func RunMeshRegistered(opts *options.ServerOptions, services ...server.Service) RunFunc {
	return RunMesh(opts, registerServices(services), buildEngine(services, server.AllHTTPRoutes()))
}

// registerServices returns a gRPC registration callback that registers each
// service's Methods (reflection) and Register (streaming) extension, or nil when
// there are no services (pure HTTP).
func registerServices(services []server.Service) func(grpc.ServiceRegistrar) {
	if len(services) == 0 {
		return nil
	}
	return func(s grpc.ServiceRegistrar) {
		for _, svc := range services {
			svc.RegisterGRPC(s)
			if svc.Register != nil {
				svc.Register(s)
			}
		}
	}
}

// buildEngine constructs the gin engine from the services' Methods/RegisterHTTP
// and the HTTPRoute plugins.
func buildEngine(services []server.Service, routes []server.HTTPRoute) *gin.Engine {
	engine := gin.New()
	for _, svc := range services {
		for _, m := range svc.Methods {
			engine.Handle(m.Method, m.Path, middleware.HTTPHandler(m.Handler, m.NewReq))
		}
		if svc.RegisterHTTP != nil {
			svc.RegisterHTTP(engine)
		}
	}
	for _, r := range routes {
		r.RegisterRoutes(engine)
	}
	return engine
}

// NewMeshServer returns a MeshServer assembled from the given options, gRPC
// service registration callback and optional gin engine.
func NewMeshServer(opts *options.ServerOptions, register func(grpc.ServiceRegistrar), engine *gin.Engine) *MeshServer {
	return &MeshServer{run: RunMesh(opts, register, engine)}
}

// NewMeshServerWithServices returns a MeshServer assembled from server.Service
// declarations, mirroring RunMeshWithServices.
func NewMeshServerWithServices(opts *options.ServerOptions, services ...server.Service) *MeshServer {
	return &MeshServer{run: RunMeshWithServices(opts, services...)}
}

// Run assembles, registers and serves the mesh service until ctx is canceled or
// a server fails, then gracefully stops and deregisters.
func (s *MeshServer) Run(ctx context.Context) error {
	return s.run(ctx)
}

// GracefulStop is an idempotent no-op: RunMesh already stops and deregisters
// before Run returns. It exists so *MeshServer satisfies the onexstack
// server.Server duck type (Run + GracefulStop).
func (s *MeshServer) GracefulStop(context.Context) error { return nil }
