// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package server defines the server lifecycle abstraction and concrete HTTP
// (gin) and gRPC servers, wired to service registration for graceful
// deregister-before-stop.
package server

import (
	"context"
	"log/slog"
	"time"
)

// Server is the unified lifecycle for all servers (HTTP/gRPC).
type Server interface {
	// Start runs the server and blocks until it stops or errors. A graceful
	// stop returns nil.
	Start(ctx context.Context) error
	// Stop gracefully shuts the server down, honoring ctx timeout.
	Stop(ctx context.Context) error
}

const defaultShutdownTimeout = 10 * time.Second

// Serve starts srv and blocks until ctx is canceled or srv exits. On
// cancellation it gracefully stops srv within a timeout budget.
func Serve(ctx context.Context, srv Server) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	if err := srv.Stop(shutdownCtx); err != nil {
		slog.Error("failed to stop server", "err", err)
	}

	return <-errCh
}
