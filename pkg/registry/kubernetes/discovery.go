// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// serviceNameLabel is the well-known label linking an EndpointSlice to its
// owning Service.
const serviceNameLabel = "kubernetes.io/service-name"

// discovery implements registry.Discovery by listing/watching EndpointSlices.
type discovery struct {
	client kubernetes.Interface
	opts   Options
}

// NewDiscovery creates a Kubernetes Discovery. When Kubeconfig is empty it
// uses in-cluster configuration (requires RBAC for list/watch endpointslices).
func NewDiscovery(opts Options) (registry.Discovery, error) {
	config, err := buildConfig(opts.Kubeconfig)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("kubernetes: create client: %w", err)
	}
	return &discovery{client: client, opts: opts}, nil
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("kubernetes: load kubeconfig: %w", err)
		}
		return cfg, nil
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("kubernetes: in-cluster config: %w", err)
	}
	return cfg, nil
}

// resolve maps a onexmesh service name to a concrete ServiceRef.
func (d *discovery) resolve(serviceName string) (ServiceRef, error) {
	ref, ok := d.opts.Services[serviceName]
	if !ok {
		return ServiceRef{}, fmt.Errorf("kubernetes: no mapping for service %q", serviceName)
	}
	if ref.Namespace == "" {
		ref.Namespace = d.opts.Namespace
	}
	if ref.Namespace == "" {
		ref.Namespace = "default"
	}
	if ref.PortName == "" {
		ref.PortName = d.opts.PortName
	}
	return ref, nil
}

func (d *discovery) listOptions(service string) metav1.ListOptions {
	return metav1.ListOptions{LabelSelector: serviceNameLabel + "=" + service}
}

func (d *discovery) GetService(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	ref, err := d.resolve(serviceName)
	if err != nil {
		return nil, err
	}
	slices, err := d.client.DiscoveryV1().EndpointSlices(ref.Namespace).List(ctx, d.listOptions(ref.Service))
	if err != nil {
		return nil, fmt.Errorf("kubernetes: list endpointslices: %w", err)
	}
	return endpointSlicesToInstances(serviceName, slices.Items, ref.PortName)
}

func (d *discovery) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	ref, err := d.resolve(serviceName)
	if err != nil {
		return nil, err
	}
	w, err := d.client.DiscoveryV1().EndpointSlices(ref.Namespace).Watch(ctx, d.listOptions(ref.Service))
	if err != nil {
		return nil, fmt.Errorf("kubernetes: watch endpointslices: %w", err)
	}
	return &watcher{
		client:      d.client,
		watch:       w,
		serviceName: serviceName,
		ref:         ref,
	}, nil
}

// endpointSlicesToInstances converts ready endpoints into instances. The port
// is selected by PortName (or the first port when empty). This assumes a
// headless Service where the EndpointSlice port equals the Pod port.
func endpointSlicesToInstances(serviceName string, slices []discoveryv1.EndpointSlice, portName string) ([]*registry.ServiceInstance, error) {
	type addrPort struct {
		addr string
		port int32
	}

	var eps []addrPort
	for _, slice := range slices {
		port, ok := selectPort(slice.Ports, portName)
		if !ok {
			continue
		}
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			for _, addr := range ep.Addresses {
				eps = append(eps, addrPort{addr: addr, port: port})
			}
		}
	}

	if portName != "" && len(eps) == 0 {
		return nil, fmt.Errorf("kubernetes: no ready endpoint with port %q for service %q", portName, serviceName)
	}

	out := make([]*registry.ServiceInstance, 0, len(eps))
	for _, ep := range eps {
		out = append(out, &registry.ServiceInstance{
			ID:        ep.addr,
			Name:      serviceName,
			Endpoints: []string{"grpc://" + net.JoinHostPort(ep.addr, strconv.Itoa(int(ep.port)))},
		})
	}
	return out, nil
}

// selectPort returns the port matching name, or the first port when name is
// empty. The bool is false when no port matches.
func selectPort(ports []discoveryv1.EndpointPort, name string) (int32, bool) {
	if len(ports) == 0 {
		return 0, false
	}
	for _, p := range ports {
		if name == "" || (p.Name != nil && *p.Name == name) {
			if p.Port != nil {
				return *p.Port, true
			}
		}
	}
	return 0, false
}

// watcher re-lists on every EndpointSlice change and returns a full snapshot.
type watcher struct {
	client      kubernetes.Interface
	watch       watch.Interface
	serviceName string
	ref         ServiceRef
}

func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	if _, ok := <-w.watch.ResultChan(); !ok {
		return nil, errors.New("kubernetes: watch closed")
	}
	slices, err := w.client.DiscoveryV1().EndpointSlices(w.ref.Namespace).List(
		context.Background(), metav1.ListOptions{LabelSelector: serviceNameLabel + "=" + w.ref.Service})
	if err != nil {
		return nil, fmt.Errorf("kubernetes: list after watch: %w", err)
	}
	return endpointSlicesToInstances(w.serviceName, slices.Items, w.ref.PortName)
}

func (w *watcher) Stop() error {
	w.watch.Stop()
	return nil
}
