// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package nacos

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// discovery implements registry.Discovery against Nacos.
type discovery struct {
	opts   Options
	client naming_client.INamingClient
}

// NewDiscovery creates a Nacos Discovery.
func NewDiscovery(opts Options) (registry.Discovery, error) {
	client, err := newNamingClient(opts)
	if err != nil {
		return nil, err
	}
	return &discovery{opts: opts, client: client}, nil
}

func (d *discovery) GetService(_ context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	instances, err := d.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   groupName(d.opts),
		Clusters:    []string{clusterName(d.opts)},
		HealthyOnly: true,
	})
	if err != nil {
		return nil, fmt.Errorf("nacos: select instances: %w", err)
	}

	out := make([]*registry.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		out = append(out, toServiceInstance(serviceName, d.opts.Protocol, inst))
	}
	return out, nil
}

func (d *discovery) Watch(_ context.Context, serviceName string) (registry.Watcher, error) {
	w := &watcher{
		name:   serviceName,
		opts:   d.opts,
		client: d.client,
		ch:     make(chan []*registry.ServiceInstance, 1),
		stop:   make(chan struct{}),
	}

	param := &vo.SubscribeParam{
		ServiceName: serviceName,
		GroupName:   groupName(d.opts),
		Clusters:    []string{clusterName(d.opts)},
		SubscribeCallback: func(services []model.SubscribeService, err error) {
			if err != nil {
				return
			}
			w.push(services)
		},
	}
	if err := d.client.Subscribe(param); err != nil {
		return nil, fmt.Errorf("nacos: subscribe: %w", err)
	}
	w.param = param
	return w, nil
}

// watcher streams full instance snapshots via the Nacos subscribe callback.
type watcher struct {
	name   string
	opts   Options
	client naming_client.INamingClient
	param  *vo.SubscribeParam

	ch   chan []*registry.ServiceInstance
	stop chan struct{}
	once sync.Once
}

func (w *watcher) push(services []model.SubscribeService) {
	out := make([]*registry.ServiceInstance, 0, len(services))
	for _, svc := range services {
		if !svc.Valid || !svc.Enable {
			continue
		}
		out = append(out, toSubscribeServiceInstance(w.name, w.opts.Protocol, svc))
	}
	select {
	case w.ch <- out:
	case <-w.stop:
	}
}

func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	select {
	case insts := <-w.ch:
		return insts, nil
	case <-w.stop:
		return nil, errors.New("nacos: watcher stopped")
	}
}

func (w *watcher) Stop() error {
	w.once.Do(func() {
		close(w.stop)
		if w.param != nil {
			_ = w.client.Unsubscribe(w.param)
		}
	})
	return nil
}

// toServiceInstance converts a nacos model.Instance into a registry.ServiceInstance.
func toServiceInstance(name, protocol string, inst model.Instance) *registry.ServiceInstance {
	if protocol == "" {
		protocol = "grpc"
	}
	endpoint := protocol + "://" + net.JoinHostPort(inst.Ip, strconv.FormatUint(inst.Port, 10))
	return &registry.ServiceInstance{
		ID:        inst.InstanceId,
		Name:      name,
		Metadata:  inst.Metadata,
		Endpoints: []string{endpoint},
	}
}

// toSubscribeServiceInstance converts a nacos SubscribeService snapshot entry.
func toSubscribeServiceInstance(name, protocol string, svc model.SubscribeService) *registry.ServiceInstance {
	if protocol == "" {
		protocol = "grpc"
	}
	endpoint := protocol + "://" + net.JoinHostPort(svc.Ip, strconv.FormatUint(svc.Port, 10))
	return &registry.ServiceInstance{
		ID:        svc.InstanceId,
		Name:      name,
		Metadata:  svc.Metadata,
		Endpoints: []string{endpoint},
	}
}
