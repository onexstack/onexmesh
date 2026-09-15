// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package consul

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// registrar implements registry.Registrar against Consul. Each instance is
// bound to a TTL health check refreshed by a heartbeat goroutine.
type registrar struct {
	opts   Options
	client *consulapi.Client

	mu        sync.Mutex
	checkID   string
	serviceID string
	stop      chan struct{}
}

// NewRegistrar creates a Consul Registrar.
func NewRegistrar(opts Options) (registry.Registrar, error) {
	client, err := newClient(opts)
	if err != nil {
		return nil, err
	}
	return &registrar{opts: opts, client: client}, nil
}

func (r *registrar) Register(_ context.Context, inst *registry.ServiceInstance) error {
	if inst == nil {
		return fmt.Errorf("consul: nil instance")
	}

	serviceID := inst.ID
	if serviceID == "" {
		serviceID = fmt.Sprintf("%s-%s-%d", inst.Name, r.opts.Host, r.opts.Port)
	}
	checkID := "service:" + serviceID

	reg := &consulapi.AgentServiceRegistration{
		ID:      serviceID,
		Name:    inst.Name,
		Address: r.opts.Host,
		Port:    r.opts.Port,
		Meta:    inst.Metadata,
		Check: &consulapi.AgentServiceCheck{
			CheckID:                        checkID,
			TTL:                            ttl(r.opts).String(),
			Status:                         consulapi.HealthPassing,
			DeregisterCriticalServiceAfter: (2 * ttl(r.opts)).String(),
		},
	}

	if err := r.client.Agent().ServiceRegister(reg); err != nil {
		return fmt.Errorf("consul: register service: %w", err)
	}

	r.mu.Lock()
	// Stop any previous heartbeat before replacing it, so re-registering does not
	// leak the old heartbeat goroutine holding a stale stop channel.
	if r.stop != nil {
		close(r.stop)
	}
	r.serviceID = serviceID
	r.checkID = checkID
	r.stop = make(chan struct{})
	stop := r.stop
	r.mu.Unlock()

	go r.heartbeat(stop, checkID)
	return nil
}

// heartbeat periodically passes the TTL check to keep the instance alive. It
// takes its stop channel and check ID as arguments so a later re-register does
// not disturb this goroutine's lifecycle.
func (r *registrar) heartbeat(stop chan struct{}, checkID string) {
	interval := ttl(r.opts) / 2
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Pass immediately so the instance is healthy right away.
	_ = r.client.Agent().UpdateTTL(checkID, "registered", consulapi.HealthPassing)

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := r.client.Agent().UpdateTTL(checkID, "ok", consulapi.HealthPassing); err != nil {
				slog.Error("consul heartbeat failed", "check_id", checkID, "err", err)
			}
		}
	}
}

func (r *registrar) Deregister(_ context.Context, inst *registry.ServiceInstance) error {
	r.mu.Lock()
	if r.stop == nil {
		r.mu.Unlock()
		return nil
	}
	serviceID := r.serviceID
	close(r.stop)
	r.stop = nil
	r.mu.Unlock()

	if err := r.client.Agent().ServiceDeregister(serviceID); err != nil {
		return fmt.Errorf("consul: deregister service: %w", err)
	}
	return nil
}
