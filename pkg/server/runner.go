// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"log/slog"
	"net"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// servable is the serve+shutdown contract shared by the gRPC and HTTP servers.
// It collapses the previously duplicated listen→register→serve→graceful-stop
// scaffold in GRPCServer.Start and HTTPServer.Start.
type servable interface {
	// Serve blocks serving accepted connections until the listener closes.
	Serve(lis net.Listener) error
	// Shutdown gracefully drains the server, honoring the ctx timeout.
	Shutdown(ctx context.Context) error
	// isClosedError reports whether err is the "already stopped" sentinel that
	// should be treated as a clean shutdown rather than a startup failure.
	isClosedError(err error) bool
}

// serve runs the shared startup scaffold: listen on addr, register to the
// registry (if configured), serve in a goroutine, and block until the server
// exits or ctx is canceled — in which case it gracefully shuts down within
// defaultShutdownTimeout. This keeps ServiceGroup able to stop siblings when one
// server fails (via errgroup.WithContext) without either server blocking forever.
func serve(ctx context.Context, s servable, reg registry.Registrar, inst *registry.ServiceInstance, addr, kind string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	// Close on early return so a registration failure does not leak the socket.
	defer lis.Close()

	if reg != nil && inst != nil {
		if err := reg.Register(ctx, inst); err != nil {
			return err
		}
	}

	slog.Info("starting "+kind+" server", "addr", addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve(lis)
	}()

	select {
	case err := <-errCh:
		if err != nil && !s.isClosedError(err) {
			return err
		}
		return nil
	case <-ctx.Done():
		slog.Info(kind+" server context canceled, shutting down", "addr", addr)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		if err := s.Shutdown(shutdownCtx); err != nil {
			slog.Error(kind+" server shutdown failed", "error", err)
		}
		return nil
	}
}
