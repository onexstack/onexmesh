// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

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

	lis net.Listener
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

func (s *GRPCServer) Start(ctx context.Context) error {
	if s.register != nil {
		s.register(s.server)
	}

	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.lis = lis

	if s.registrar != nil && s.instance != nil {
		if err := s.registrar.Register(ctx, s.instance); err != nil {
			_ = lis.Close()
			return err
		}
	}

	slog.Info("starting grpc server", "addr", s.addr)

	// Serve in a goroutine so Start can respond to context cancellation and
	// sibling failure. ServiceGroup drives Start via errgroup.WithContext: when
	// one server fails and cancels ctx, every sibling must return instead of
	// blocking forever in Serve.
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.Serve(lis)
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return err
		}
		return nil
	case <-ctx.Done():
		slog.Info("grpc server context canceled, shutting down", "addr", s.addr)
		done := make(chan struct{})
		go func() {
			s.server.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(defaultShutdownTimeout):
			slog.Warn("grpc server graceful stop timed out, forcing stop", "addr", s.addr)
			s.server.Stop()
			<-done
		}
		return nil
	}
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
