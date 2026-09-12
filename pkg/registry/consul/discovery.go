// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package consul

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/onexstack/onexmesh/pkg/registry"
)

const pollInterval = 2 * time.Second

// discovery implements registry.Discovery against Consul.
type discovery struct {
	opts   Options
	client *consulapi.Client
}

// NewDiscovery creates a Consul Discovery.
func NewDiscovery(opts Options) (registry.Discovery, error) {
	client, err := newClient(opts)
	if err != nil {
		return nil, err
	}
	return &discovery{opts: opts, client: client}, nil
}

func (d *discovery) GetService(_ context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	entries, _, err := d.client.Health().Service(serviceName, "", true, &consulapi.QueryOptions{
		Namespace: d.opts.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("consul: get service: %w", err)
	}
	return entriesToInstances(serviceName, d.opts.Protocol, entries), nil
}

func (d *discovery) Watch(_ context.Context, serviceName string) (registry.Watcher, error) {
	return &watcher{
		client:      d.client,
		serviceName: serviceName,
		opts:        d.opts,
		stop:        make(chan struct{}),
	}, nil
}

// watcher polls Consul health and emits a snapshot whenever the modify index
// changes, adapting the blocking-query semantics to a simple poll loop.
type watcher struct {
	client      *consulapi.Client
	serviceName string
	opts        Options

	stop chan struct{}
	once sync.Once
}

func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastIndex uint64
	for {
		select {
		case <-w.stop:
			return nil, errors.New("consul: watcher stopped")
		case <-ticker.C:
			entries, meta, err := w.client.Health().Service(w.serviceName, "", true, &consulapi.QueryOptions{
				Namespace: w.opts.Namespace,
			})
			if err != nil {
				continue
			}
			if meta.LastIndex == lastIndex {
				continue
			}
			lastIndex = meta.LastIndex
			return entriesToInstances(w.serviceName, w.opts.Protocol, entries), nil
		}
	}
}

func (w *watcher) Stop() error {
	w.once.Do(func() { close(w.stop) })
	return nil
}

// entriesToInstances converts consul service entries into instances.
func entriesToInstances(name, protocol string, entries []*consulapi.ServiceEntry) []*registry.ServiceInstance {
	if protocol == "" {
		protocol = "grpc"
	}
	out := make([]*registry.ServiceInstance, 0, len(entries))
	for _, e := range entries {
		addr := e.Service.Address
		if addr == "" && e.Node != nil {
			addr = e.Node.Address
		}
		endpoint := protocol + "://" + net.JoinHostPort(addr, strconv.Itoa(e.Service.Port))
		out = append(out, &registry.ServiceInstance{
			ID:        e.Service.ID,
			Name:      name,
			Metadata:  e.Service.Meta,
			Endpoints: []string{endpoint},
		})
	}
	return out
}
