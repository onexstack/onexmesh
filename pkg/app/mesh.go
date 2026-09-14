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

// resolveMiddleware assembles the server middleware chain, outermost first. It
// prefers the route-aware matcher when route-level bindings are configured,
// otherwise the flat global chain. RunMesh and newEngine share it so the gRPC
// interceptors and gin handlers are built from the same source.
func resolveMiddleware(opts *options.ServerOptions) ([]middleware.Middleware, error) {
	if m, err := opts.BuildMatcher(); err != nil {
		return nil, err
	} else if m != nil {
		return []middleware.Middleware{matcher.Match(m)}, nil
	}
	return opts.BuildMiddleware(), nil
}

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
		if engine != nil && !isMeshEngine(engine) {
			return fmt.Errorf("app: gin engine must be built via app.NewEngine (a raw gin.New() would bypass the unified middleware chain)")
		}

		// Resolve the middleware chain once — route-aware via the matcher when
		// configured, otherwise the flat global chain — then adapt it to both the
		// unary and stream gRPC bridges and the gin bridge from the same source.
		mws, err := resolveMiddleware(opts)
		if err != nil {
			return err
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
			// The unified chain is already applied by newEngine; Use() here would wrap
			// the routes a second time.
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
	host, port, err := splitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("parse %s listen address %q: %w", protocol, addr, err)
	}
	registrar, err := opts.Registry.NewRegistrar(host, port, protocol)
	if err != nil {
		return nil, fmt.Errorf("build registrar: %w", err)
	}
	return registrar, nil
}

// splitHostPort splits a listen "host:port" address into its components. It uses
// net.SplitHostPort so it handles bracketed IPv6 literals, which the Nacos
// backend's local splitHostPort (a LastIndex split for server addresses)
// intentionally does not. A malformed address or non-numeric port is an error
// rather than a silent port-0 registration.
func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q", portStr)
	}
	return host, port, nil
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
// services. Each Service.Methods declares a proto-first HTTP route whose Handler
// is the same strongly-typed function the gRPC service exposes via
// Service.Register, so one implementation serves both protocols. Service.Register
// carries the gRPC registration (generated RegisterXxxServer plus streaming);
// Service.RouteGroups is the native-gin escape hatch. It delegates to RunMesh, so
// middleware, registration and lifecycle behavior are identical.
func RunMeshWithServices(opts *options.ServerOptions, services ...server.Service) RunFunc {
	engine, err := buildEngine(opts, services, nil)
	if err != nil {
		return func(context.Context) error { return err }
	}
	return RunMesh(opts, registerServices(services), engine)
}

// RunMeshWithRoutes assembles a pure HTTP service from explicit HTTPRoute
// plugins (no gRPC methods). Each route registers itself onto the gin engine;
// RunMesh wires the standard middleware chain, registration and lifecycle.
func RunMeshWithRoutes(opts *options.ServerOptions, routes ...server.HTTPRoute) RunFunc {
	engine, err := buildEngine(opts, nil, routes)
	if err != nil {
		return func(context.Context) error { return err }
	}
	return RunMesh(opts, nil, engine)
}

// RunMeshWithEngine assembles a dual-protocol service from an externally-built
// engine (via app.NewEngine) plus optional proto-first services. Unlike
// RunMeshWithServices/RunMeshRegistered, the caller owns the gin engine and may
// register native gin routes directly (engine.Group / GET / POST / ...); the
// services' proto-first Methods and RouteGroups are then applied on top of the
// engine's root router group, and their gRPC methods are registered.
// opts.Mesh.Protocol still gates which protocols actually serve. As with
// RunMeshHTTP, the engine must be built with app.NewEngine using the final opts
// so the unified middleware chain is injected before any route is registered.
func RunMeshWithEngine(opts *options.ServerOptions, engine *gin.Engine, services ...server.Service) RunFunc {
	if engine == nil {
		return func(context.Context) error {
			return fmt.Errorf("app: RunMeshWithEngine requires a non-nil engine built via app.NewEngine")
		}
	}
	for _, svc := range services {
		for _, m := range svc.Methods {
			m.Apply(&engine.RouterGroup)
		}
		for _, g := range svc.RouteGroups {
			g.Apply(&engine.RouterGroup)
		}
	}
	return RunMesh(opts, registerServices(services), engine)
}

// RunMeshRegistered auto-discovers every HTTPRoute registered via
// server.RegisterHTTPRoute and assembles them alongside any explicit services.
// Business packages self-register in init(), so the composition root needs no
// explicit import of each route module.
func RunMeshRegistered(opts *options.ServerOptions, services ...server.Service) RunFunc {
	engine, err := buildEngine(opts, services, server.AllHTTPRoutes())
	if err != nil {
		return func(context.Context) error { return err }
	}
	return RunMesh(opts, registerServices(services), engine)
}

// RunMeshGRPC serves only gRPC: it registers the given services and skips the
// gin engine. opts.Mesh.Protocol must include "grpc".
func RunMeshGRPC(opts *options.ServerOptions, register func(grpc.ServiceRegistrar)) RunFunc {
	return RunMesh(opts, register, nil)
}

// RunMeshHTTP serves only gin over HTTP: the engine must be built via
// app.NewEngine so the unified middleware chain is applied. opts.Mesh.Protocol
// must include "http".
func RunMeshHTTP(opts *options.ServerOptions, engine *gin.Engine) RunFunc {
	return RunMesh(opts, nil, engine)
}

// registerServices returns a gRPC registration callback that registers each
// service's gRPC methods via its Register extension (generated RegisterXxxServer
// plus streaming), or nil when there are no services (pure HTTP).
func registerServices(services []server.Service) func(grpc.ServiceRegistrar) {
	if len(services) == 0 {
		return nil
	}
	return func(s grpc.ServiceRegistrar) {
		for _, svc := range services {
			if svc.Register != nil {
				svc.Register(s)
			}
		}
	}
}

// buildEngine constructs the gin engine from the services' proto-first Methods
// and RouteGroups and the HTTPRoute plugins, each applied to the engine's root
// router group.
func buildEngine(opts *options.ServerOptions, services []server.Service, routes []server.HTTPRoute) (*gin.Engine, error) {
	engine, err := newEngine(opts)
	if err != nil {
		return nil, err
	}
	root := &engine.RouterGroup
	for _, svc := range services {
		for _, m := range svc.Methods {
			m.Apply(root)
		}
		for _, g := range svc.RouteGroups {
			g.Apply(root)
		}
	}
	for _, r := range routes {
		for _, g := range r.RouteGroups() {
			g.Apply(root)
		}
	}
	return engine, nil
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
