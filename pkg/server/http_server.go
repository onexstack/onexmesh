// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// HTTPServer is a gin-backed HTTP server with optional service registration.
type HTTPServer struct {
	addr      string
	handler   http.Handler
	registrar registry.Registrar
	instance  *registry.ServiceInstance

	server *http.Server
	lis    net.Listener
}

// HTTPOption configures an HTTPServer.
type HTTPOption func(*HTTPServer)

// WithRegistrar attaches a registrar and the instance to register on Start and
// deregister on Stop.
func WithRegistrar(r registry.Registrar, inst *registry.ServiceInstance) HTTPOption {
	return func(s *HTTPServer) {
		s.registrar = r
		s.instance = inst
	}
}

// NewHTTPServer creates an HTTP server serving the given handler.
func NewHTTPServer(addr string, handler http.Handler, opts ...HTTPOption) *HTTPServer {
	s := &HTTPServer{addr: addr, handler: handler}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *HTTPServer) Start(ctx context.Context) error {
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

	s.server = &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	slog.Info("starting http server", "addr", s.addr)

	// Serve in a goroutine so Start can respond to context cancellation and
	// sibling failure (see GRPCServer.Start for the errgroup rationale).
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.Serve(lis)
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		slog.Info("http server context canceled, shutting down", "addr", s.addr)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			slog.Error("http server shutdown failed", "error", err)
		}
		return nil
	}
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	if s.registrar != nil && s.instance != nil {
		if err := s.registrar.Deregister(ctx, s.instance); err != nil {
			slog.Error("failed to deregister http server", "err", err)
		}
	}
	if s.server == nil {
		return nil
	}
	slog.Info("stopping http server")
	return s.server.Shutdown(ctx)
}
