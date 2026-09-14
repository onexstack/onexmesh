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

// httpServable adapts *http.Server to the shared servable contract.
type httpServable struct{ *http.Server }

func (h httpServable) Serve(lis net.Listener) error { return h.Server.Serve(lis) }
func (h httpServable) Shutdown(ctx context.Context) error {
	return h.Server.Shutdown(ctx)
}
func (h httpServable) isClosedError(err error) bool { return errors.Is(err, http.ErrServerClosed) }

func (s *HTTPServer) Start(ctx context.Context) error {
	s.server = &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	return serve(ctx, httpServable{s.server}, s.registrar, s.instance, s.addr, "http")
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
