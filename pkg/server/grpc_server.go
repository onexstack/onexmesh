// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"google.golang.org/grpc"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// GRPCServer is a grpc-go server with optional service registration.
type GRPCServer struct {
	addr      string
	server    *grpc.Server
	register  func(grpc.ServiceRegistrar)
	registrar registry.Registrar
	instance  *registry.ServiceInstance
}

// GRPCOption configures a GRPCServer.
type GRPCOption func(*GRPCServer)

// WithGRPCRegistrar attaches a registrar and the instance to register on Start
// and deregister on Stop.
func WithGRPCRegistrar(r registry.Registrar, inst *registry.ServiceInstance) GRPCOption {
	return func(s *GRPCServer) {
		s.registrar = r
		s.instance = inst
	}
}

// NewGRPCServer creates a gRPC server. register is invoked with the server on
// Start to register business services.
func NewGRPCServer(addr string, register func(grpc.ServiceRegistrar), opts ...grpc.ServerOption) *GRPCServer {
	return &GRPCServer{
		addr:     addr,
		server:   grpc.NewServer(opts...),
		register: register,
	}
}

// WithRegistrar is a fluent variant of WithGRPCRegistrar.
func (s *GRPCServer) WithRegistrar(r registry.Registrar, inst *registry.ServiceInstance) *GRPCServer {
	s.registrar = r
	s.instance = inst
	return s
}

// grpcServable adapts *grpc.Server to the shared servable contract.
type grpcServable struct{ *grpc.Server }

func (g grpcServable) Serve(lis net.Listener) error { return g.Server.Serve(lis) }

func (g grpcServable) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		g.Server.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		g.Server.Stop()
		<-done
		return ctx.Err()
	}
}

func (g grpcServable) isClosedError(err error) bool { return errors.Is(err, grpc.ErrServerStopped) }

func (s *GRPCServer) Start(ctx context.Context) error {
	if s.register != nil {
		s.register(s.server)
	}
	return serve(ctx, grpcServable{s.server}, s.registrar, s.instance, s.addr, "grpc")
}

func (s *GRPCServer) Stop(ctx context.Context) error {
	if s.registrar != nil && s.instance != nil {
		if err := s.registrar.Deregister(ctx, s.instance); err != nil {
			slog.Error("failed to deregister grpc server", "err", err)
		}
	}

	slog.Info("stopping grpc server")
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		slog.Warn("grpc server graceful stop timed out, forcing stop", "err", ctx.Err())
		s.server.Stop()
		<-done
		return ctx.Err()
	}
}
