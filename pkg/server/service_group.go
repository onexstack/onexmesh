// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ServiceEntry couples a name with a Server for lifecycle management.
type ServiceEntry struct {
	Name   string
	Server Server
}

// ServiceGroup manages a set of servers: they start concurrently and stop in
// reverse registration order (later-started services stop first), following
// the go-zero ServiceGroup convention.
type ServiceGroup struct {
	mu       sync.Mutex
	services []ServiceEntry
}

// NewServiceGroup creates an empty ServiceGroup.
func NewServiceGroup() *ServiceGroup {
	return &ServiceGroup{}
}

// Add appends a server to the group.
func (g *ServiceGroup) Add(name string, s Server) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.services = append(g.services, ServiceEntry{Name: name, Server: s})
}

// Start starts all servers concurrently and blocks until one fails or the
// context is canceled.
func (g *ServiceGroup) Start(ctx context.Context) error {
	g.mu.Lock()
	services := append([]ServiceEntry(nil), g.services...)
	g.mu.Unlock()

	eg, ctx := errgroup.WithContext(ctx)
	for _, svc := range services {
		svc := svc
		eg.Go(func() error {
			if err := svc.Server.Start(ctx); err != nil {
				return fmt.Errorf("%s: %w", svc.Name, err)
			}
			return nil
		})
	}
	return eg.Wait()
}

// Stop stops all servers in reverse registration order, serially. It attempts
// to stop every server and aggregates any errors, so one failing stop does not
// leak the remaining servers.
func (g *ServiceGroup) Stop(ctx context.Context) error {
	g.mu.Lock()
	services := append([]ServiceEntry(nil), g.services...)
	g.mu.Unlock()

	var errs []error
	for i := len(services) - 1; i >= 0; i-- {
		if err := services[i].Server.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", services[i].Name, err))
		}
	}
	return errors.Join(errs...)
}
