// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package polaris

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// discovery implements registry.Discovery against Polaris.
type discovery struct {
	opts     Options
	consumer api.ConsumerAPI
}

// NewDiscovery creates a Polaris Discovery.
func NewDiscovery(opts Options) (registry.Discovery, error) {
	consumer, err := api.NewConsumerAPIByAddress(opts.Addr)
	if err != nil {
		return nil, fmt.Errorf("polaris: create consumer api: %w", err)
	}
	return &discovery{opts: opts, consumer: consumer}, nil
}

func (d *discovery) GetService(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	resp, err := d.consumer.GetInstances(&api.GetInstancesRequest{
		GetInstancesRequest: model.GetInstancesRequest{
			Service:   serviceName,
			Namespace: d.opts.Namespace,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("polaris: get instances: %w", err)
	}

	out := make([]*registry.ServiceInstance, 0, len(resp.Instances))
	for _, inst := range resp.Instances {
		if !inst.IsHealthy() || inst.IsIsolated() {
			continue
		}
		out = append(out, toServiceInstance(serviceName, inst))
	}
	return out, nil
}

func (d *discovery) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	resp, err := d.consumer.WatchService(&api.WatchServiceRequest{
		WatchServiceRequest: model.WatchServiceRequest{
			Key: model.ServiceKey{Namespace: d.opts.Namespace, Service: serviceName},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("polaris: watch service: %w", err)
	}
	return newWatcher(serviceName, d.consumer, resp), nil
}

// toServiceInstance converts a polaris Instance into a registry.ServiceInstance.
func toServiceInstance(name string, inst model.Instance) *registry.ServiceInstance {
	protocol := inst.GetProtocol()
	if protocol == "" {
		protocol = "grpc"
	}
	endpoint := protocol + "://" + net.JoinHostPort(inst.GetHost(), strconv.Itoa(int(inst.GetPort())))
	return &registry.ServiceInstance{
		ID:        inst.GetId(),
		Name:      name,
		Version:   inst.GetVersion(),
		Metadata:  inst.GetMetadata(),
		Endpoints: []string{endpoint},
	}
}

// watcher streams instance snapshots for a watched Polaris service. The first
// Next returns the initial full snapshot; subsequent calls apply incremental
// add/update/delete events and return the refreshed snapshot.
type watcher struct {
	name     string
	consumer api.ConsumerAPI
	resp     *model.WatchServiceResponse

	mu      sync.Mutex
	current []*registry.ServiceInstance
	init    bool
	once    sync.Once
}

func newWatcher(name string, consumer api.ConsumerAPI, resp *model.WatchServiceResponse) *watcher {
	return &watcher{name: name, consumer: consumer, resp: resp}
}

func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.init {
		w.init = true
		w.current = instancesFromSlice(w.name, w.resp.GetAllInstancesResp.GetInstances())
		return clone(w.current), nil
	}

	event, ok := <-w.resp.EventChannel
	if !ok {
		return nil, errors.New("polaris: watch channel closed")
	}
	w.applyEvent(event)
	return clone(w.current), nil
}

func (w *watcher) Stop() error {
	w.once.Do(func() { w.consumer.Destroy() })
	return nil
}

// applyEvent updates the current snapshot from a polaris subscription event.
func (w *watcher) applyEvent(e model.SubScribeEvent) {
	ev, ok := e.(*model.InstanceEvent)
	if !ok {
		return
	}

	index := make(map[string]*registry.ServiceInstance, len(w.current))
	for _, si := range w.current {
		index[si.ID] = si
	}

	if ev.AddEvent != nil {
		for _, inst := range ev.AddEvent.Instances {
			si := toServiceInstance(w.name, inst)
			index[si.ID] = si
		}
	}
	if ev.UpdateEvent != nil {
		for _, u := range ev.UpdateEvent.UpdateList {
			si := toServiceInstance(w.name, u.After)
			index[si.ID] = si
		}
	}
	if ev.DeleteEvent != nil {
		for _, inst := range ev.DeleteEvent.Instances {
			delete(index, inst.GetId())
		}
	}

	w.current = make([]*registry.ServiceInstance, 0, len(index))
	for _, si := range index {
		w.current = append(w.current, si)
	}
}

func instancesFromSlice(name string, in []model.Instance) []*registry.ServiceInstance {
	out := make([]*registry.ServiceInstance, 0, len(in))
	for _, inst := range in {
		if !inst.IsHealthy() || inst.IsIsolated() {
			continue
		}
		out = append(out, toServiceInstance(name, inst))
	}
	return out
}

func clone(in []*registry.ServiceInstance) []*registry.ServiceInstance {
	out := make([]*registry.ServiceInstance, len(in))
	copy(out, in)
	return out
}
