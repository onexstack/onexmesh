// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package nacos

import (
	"context"
	"fmt"

	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/vo"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// registrar implements registry.Registrar against Nacos. Ephemeral instances
// are heartbeat by the SDK automatically.
type registrar struct {
	opts   Options
	client naming_client.INamingClient
}

// NewRegistrar creates a Nacos Registrar.
func NewRegistrar(opts Options) (registry.Registrar, error) {
	client, err := newNamingClient(opts)
	if err != nil {
		return nil, err
	}
	return &registrar{opts: opts, client: client}, nil
}

func (r *registrar) Register(_ context.Context, inst *registry.ServiceInstance) error {
	if inst == nil {
		return fmt.Errorf("nacos: nil instance")
	}

	weight := r.opts.Weight
	if weight <= 0 {
		weight = 100
	}

	ok, err := r.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          r.opts.Host,
		Port:        uint64(r.opts.Port),
		Weight:      weight,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    inst.Metadata,
		ClusterName: clusterName(r.opts),
		ServiceName: inst.Name,
		GroupName:   groupName(r.opts),
	})
	if err != nil {
		return fmt.Errorf("nacos: register instance: %w", err)
	}
	if !ok {
		return fmt.Errorf("nacos: register instance failed")
	}
	return nil
}

func (r *registrar) Deregister(_ context.Context, inst *registry.ServiceInstance) error {
	if inst == nil {
		return fmt.Errorf("nacos: nil instance")
	}

	ok, err := r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          r.opts.Host,
		Port:        uint64(r.opts.Port),
		Cluster:     clusterName(r.opts),
		ServiceName: inst.Name,
		GroupName:   groupName(r.opts),
		Ephemeral:   true,
	})
	if err != nil {
		return fmt.Errorf("nacos: deregister instance: %w", err)
	}
	if !ok {
		return fmt.Errorf("nacos: deregister instance failed")
	}
	return nil
}
