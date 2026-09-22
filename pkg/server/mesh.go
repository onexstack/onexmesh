// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"github.com/onexstack/onexmesh/pkg/middleware"
	"github.com/onexstack/onexmesh/pkg/options"
	"github.com/onexstack/onexmesh/pkg/registry"
)

// MeshServer is the facade for a onexmesh service instance. It aggregates the
// HTTP and gRPC servers, the unified middleware chain and the registry into a
// single runnable unit, and owns the runtime initialization (slog + OpenTelemetry)
// for the options it is built from. Configure it with the WithXxx options, then
// run it via Run; a concurrent or out-of-band shutdown is available via
// GracefulStop.
type MeshServer struct {
	opts     *options.ServerOptions
	services []Service
	routes   []HTTPRoute
	engine   *gin.Engine
	register func(grpc.ServiceRegistrar)

	mu     sync.Mutex
	group  *ServiceGroup
	stop   sync.Once
	cancel context.CancelFunc
	done   chan struct{}
}

// MeshServerOption configures a MeshServer.
type MeshServerOption func(*MeshServer)

// WithService appends proto-first services. Each Service declares both the gRPC
// registration (Service.Register) and the proto-first HTTP routes (Service.Methods),
// so one implementation serves both protocols.
func WithService(svcs ...Service) MeshServerOption {
	return func(s *MeshServer) { s.services = append(s.services, svcs...) }
}

// WithRoute appends native-gin HTTP route plugins (HTTPRoute). Useful for
// path/query/streaming endpoints with no proto definition.
func WithRoute(routes ...HTTPRoute) MeshServerOption {
	return func(s *MeshServer) { s.routes = append(s.routes, routes...) }
}

// WithGinEngine supplies an externally-built gin engine (via NewGinEngine) so the caller
// can register native gin routes directly. Any WithService services are then
// applied on top of the engine's root router group.
func WithGinEngine(engine *gin.Engine) MeshServerOption {
	return func(s *MeshServer) { s.engine = engine }
}

// WithGRPCRegister supplies a raw gRPC service registration callback (the
// generated RegisterXxxServer). Prefer WithService, which carries this for you.
func WithGRPCRegister(fn func(grpc.ServiceRegistrar)) MeshServerOption {
	return func(s *MeshServer) { s.register = fn }
}

// NewMeshServer builds a MeshServer from the given options and configuration
// options. It does not initialize or serve anything; assembly and runtime
// initialization happen on Run.
func NewMeshServer(opts *options.ServerOptions, mopts ...MeshServerOption) *MeshServer {
	s := &MeshServer{opts: opts}
	for _, o := range mopts {
		o(s)
	}
	return s
}

// Options returns the server options this instance was built from.
func (s *MeshServer) Options() *options.ServerOptions { return s.opts }

// Run initializes the runtime capabilities (slog + OpenTelemetry), assembles the
// HTTP/gRPC service group, starts it and blocks until ctx is canceled or a server
// fails, then gracefully stops and deregisters the servers before returning. It
// mirrors onexstack's GenericAPIServer.Run contract, so its method value satisfies
// onexstack's app.RunFunc.
func (s *MeshServer) Run(ctx context.Context) error {
	if err := s.opts.Apply(); err != nil {
		return fmt.Errorf("apply runtime: %w", err)
	}
	defer s.shutdown()

	group, err := s.build()
	if err != nil {
		return err
	}

	// Publish the cancel/done hooks before Start so an out-of-band GracefulStop
	// that races this window cancels the run context instead of consuming the
	// shared stop.Once before any server has registered. Deregistration is then
	// performed only by the cleanup path below, so it is never skipped.
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.mu.Lock()
	s.group = group
	s.cancel = cancel
	s.done = done
	s.mu.Unlock()

	startErr := group.Start(runCtx)

	// Gracefully stop and deregister remaining servers, whether Start returned due
	// to cancellation or a sibling failure. stopGroup is shared with GracefulStop
	// so the two never double-stop the same group.
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancelShutdown()
	stopErr := s.stopGroup(shutdownCtx)

	// Clear the runtime hooks and signal completion so a waiting GracefulStop
	// unblocks only after deregistration has run; a later GracefulStop is a no-op.
	s.mu.Lock()
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()
	close(done)

	if stopErr != nil {
		return fmt.Errorf("stop service group: %w", stopErr)
	}
	return startErr
}

// GracefulStop gracefully stops the running service group by canceling its run
// context and waiting for Run's cleanup (including deregistration) to finish,
// honoring ctx timeout. It is safe to call before Run (no-op) or concurrently
// with Run.
func (s *MeshServer) GracefulStop(ctx context.Context) error {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.mu.Unlock()

	if cancel == nil {
		return nil
	}
	cancel()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// stopGroup stops the assembled service group exactly once, regardless of whether
// the call originates from Run's cleanup or an out-of-band GracefulStop.
func (s *MeshServer) stopGroup(ctx context.Context) error {
	s.mu.Lock()
	group := s.group
	s.mu.Unlock()
	if group == nil {
		return nil
	}
	var err error
	s.stop.Do(func() { err = group.Stop(ctx) })
	return err
}

// shutdown releases the runtime capabilities (OTel providers, output files)
// initialized by Run.
func (s *MeshServer) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	if err := s.opts.Shutdown(ctx); err != nil {
		slog.Error("runtime shutdown failed", "err", err)
	}
}

// build assembles the ServiceGroup from the options and configuration: it resolves
// the effective gin engine and gRPC registration, then builds the grpc/http servers
// with the unified middleware chain and their registrars.
func (s *MeshServer) build() (*ServiceGroup, error) {
	opts := s.opts

	engine, err := s.resolveEngine()
	if err != nil {
		return nil, err
	}
	register := s.register
	if register == nil {
		register = registerServices(s.services)
	}

	mws, err := resolveMiddleware(opts)
	if err != nil {
		return nil, err
	}
	var unaryInts []grpc.UnaryServerInterceptor
	var streamInts []grpc.StreamServerInterceptor
	for _, mw := range mws {
		unaryInts = append(unaryInts, middleware.UnaryServerInterceptor(mw))
		streamInts = append(streamInts, middleware.StreamServerInterceptor(mw))
	}

	group := NewServiceGroup()

	if register != nil && options.ProtocolUsesGRPC(opts.Mesh.Protocol) {
		grpcReg, err := buildRegistrarFor(opts, "grpc", opts.Mesh.GRPCAddr)
		if err != nil {
			return nil, err
		}
		grpcSrv := NewGRPCServer(
			opts.Mesh.GRPCAddr, register,
			grpc.ChainUnaryInterceptor(unaryInts...),
			grpc.ChainStreamInterceptor(streamInts...),
		)
		if grpcReg != nil {
			inst, err := opts.ServiceInstanceFor("grpc")
			if err != nil {
				return nil, err
			}
			grpcSrv.WithRegistrar(grpcReg, inst)
		}
		group.Add("grpc", grpcSrv)
	}

	if engine != nil && options.ProtocolUsesHTTP(opts.Mesh.Protocol) {
		// The unified chain is already applied by newEngine; Use() here would wrap
		// the routes a second time.
		httpReg, err := buildRegistrarFor(opts, "http", opts.Mesh.HTTPAddr)
		if err != nil {
			return nil, err
		}
		var httpOpts []HTTPOption
		if httpReg != nil {
			inst, err := opts.ServiceInstanceFor("http")
			if err != nil {
				return nil, err
			}
			httpOpts = append(httpOpts, WithRegistrar(httpReg, inst))
		}
		group.Add("http", NewHTTPServer(opts.Mesh.HTTPAddr, engine, httpOpts...))
	}

	return group, nil
}

// resolveEngine returns the gin engine to serve, applying any configured services
// and routes. An externally supplied engine is validated and services are applied
// on top of it; otherwise an engine is built from the declared services/routes.
// It returns nil for a pure-gRPC service (no engine, no services, no routes).
func (s *MeshServer) resolveEngine() (*gin.Engine, error) {
	if s.engine != nil {
		if !isMeshEngine(s.engine) {
			return nil, fmt.Errorf("server: gin engine must be built via server.NewGinEngine (a raw gin.New() would bypass the unified middleware chain)")
		}
		root := &s.engine.RouterGroup
		for _, svc := range s.services {
			for _, m := range svc.Methods {
				m.Apply(root)
			}
			for _, g := range svc.RouteGroups {
				g.Apply(root)
			}
		}
		for _, r := range s.routes {
			for _, g := range r.RouteGroups() {
				g.Apply(root)
			}
		}
		return s.engine, nil
	}

	if len(s.services) == 0 && len(s.routes) == 0 {
		return nil, nil
	}
	return buildEngine(s.opts, s.services, s.routes)
}

// buildRegistrarFor resolves the address this instance registers under and
// creates the registrar selected by the registry options. It returns (nil, nil)
// when the registry type is "none".
//
// The advertised host and port come from MeshOptions.AdvertiseAddr, which is
// also what builds the ServiceInstance's endpoints — so a backend that
// registers the address (polaris) and one that registers the instance (etcd)
// publish the same reachable address rather than the listen address.
func buildRegistrarFor(opts *options.ServerOptions, protocol, addr string) (registry.Registrar, error) {
	host, port, err := opts.Mesh.AdvertiseAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", protocol, err)
	}
	registrar, err := opts.Registry.NewRegistrar(host, port, protocol)
	if err != nil {
		return nil, fmt.Errorf("build registrar: %w", err)
	}
	return registrar, nil
}

// registerServices returns a gRPC registration callback that registers each
// service's gRPC methods via its Register extension (generated RegisterXxxServer
// plus streaming), or nil when there are no services (pure HTTP).
func registerServices(services []Service) func(grpc.ServiceRegistrar) {
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
func buildEngine(opts *options.ServerOptions, services []Service, routes []HTTPRoute) (*gin.Engine, error) {
	engine, err := newGinEngine(opts)
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
