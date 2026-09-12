// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package polaris

import (
	"context"
	"fmt"
	"sync"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// registrar implements registry.Registrar against Polaris. Heartbeat is
// delegated to the SDK via AutoHeartbeat, so no goroutine is leaked.
type registrar struct {
	opts     Options
	provider api.ProviderAPI

	mu         sync.Mutex
	instanceID string
	registered bool
}

// NewRegistrar creates a Polaris Registrar.
func NewRegistrar(opts Options) (registry.Registrar, error) {
	provider, err := api.NewProviderAPIByAddress(opts.Addr)
	if err != nil {
		return nil, fmt.Errorf("polaris: create provider api: %w", err)
	}
	return &registrar{opts: opts, provider: provider}, nil
}

func (r *registrar) Register(ctx context.Context, inst *registry.ServiceInstance) error {
	if inst == nil {
		return fmt.Errorf("polaris: nil instance")
	}

	req := &model.InstanceRegisterRequest{
		Service:      inst.Name,
		Namespace:    r.opts.Namespace,
		Host:         r.opts.Host,
		Port:         r.opts.Port,
		ServiceToken: r.opts.Token,
		Metadata:     inst.Metadata,
	}
	if r.opts.Protocol != "" {
		p := r.opts.Protocol
		req.Protocol = &p
	}
	if r.opts.Version != "" {
		v := r.opts.Version
		req.Version = &v
	}
	if r.opts.Heartbeat {
		ttl := r.opts.TTL
		req.TTL = &ttl
		req.AutoHeartbeat = true
	}

	resp, err := r.provider.RegisterInstance(&api.InstanceRegisterRequest{
		InstanceRegisterRequest: *req,
	})
	if err != nil {
		return fmt.Errorf("polaris: register instance: %w", err)
	}

	r.mu.Lock()
	r.instanceID = resp.InstanceID
	r.registered = true
	r.mu.Unlock()
	return nil
}

func (r *registrar) Deregister(ctx context.Context, inst *registry.ServiceInstance) error {
	r.mu.Lock()
	if !r.registered {
		r.mu.Unlock()
		return nil
	}
	instanceID := r.instanceID
	r.registered = false
	r.mu.Unlock()

	err := r.provider.Deregister(&api.InstanceDeRegisterRequest{
		InstanceDeRegisterRequest: model.InstanceDeRegisterRequest{
			Service:      inst.Name,
			Namespace:    r.opts.Namespace,
			Host:         r.opts.Host,
			Port:         r.opts.Port,
			ServiceToken: r.opts.Token,
			InstanceID:   instanceID,
		},
	})
	if err != nil {
		return fmt.Errorf("polaris: deregister instance: %w", err)
	}

	// Release the SDK context after the instance is removed.
	r.provider.Destroy()
	return nil
}
