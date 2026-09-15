// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package eureka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/onexstack/onexmesh/pkg/registry"
)

const pollInterval = 2 * time.Second

// discovery implements registry.Discovery against Eureka.
type discovery struct {
	opts Options
	http *http.Client
}

// NewDiscovery creates an Eureka Discovery.
func NewDiscovery(opts Options) (registry.Discovery, error) {
	return &discovery{
		opts: opts,
		http: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

func (d *discovery) GetService(_ context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	instances, err := d.fetch(serviceName)
	if err != nil {
		return nil, err
	}
	return instancesToServiceInstances(serviceName, d.opts.Protocol, instances), nil
}

// Close releases idle HTTP connections held by the discovery client.
func (d *discovery) Close() error {
	d.http.CloseIdleConnections()
	return nil
}

func (d *discovery) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	watchCtx, cancel := context.WithCancel(ctx)
	return &watcher{
		discovery:   d,
		serviceName: serviceName,
		ctx:         watchCtx,
		cancel:      cancel,
		stop:        make(chan struct{}),
	}, nil
}

// fetch retrieves the raw instance list for a service from Eureka.
func (d *discovery) fetch(serviceName string) ([]eurekaInstance, error) {
	url := fmt.Sprintf("%s/apps/%s", serverURL(d.opts), serviceName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eureka: get service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eureka: get service returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apps eurekaApplications
	if err := json.Unmarshal(data, &apps); err != nil {
		return nil, fmt.Errorf("eureka: decode service: %w", err)
	}

	for _, app := range apps.Applications.Application {
		if app.Name == serviceName {
			return app.Instance, nil
		}
	}
	return nil, nil
}

// watcher polls Eureka and emits a snapshot when the instance set changes.
type watcher struct {
	discovery   *discovery
	serviceName string

	ctx    context.Context
	cancel context.CancelFunc
	stop   chan struct{}
	once   sync.Once
}

func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var last string
	for {
		select {
		case <-w.ctx.Done():
			return nil, w.ctx.Err()
		case <-w.stop:
			return nil, errors.New("eureka: watcher stopped")
		case <-ticker.C:
			instances, err := w.discovery.fetch(w.serviceName)
			if err != nil {
				continue
			}
			cur := fingerprint(instances)
			if cur == last {
				continue
			}
			last = cur
			return instancesToServiceInstances(w.serviceName, w.discovery.opts.Protocol, instances), nil
		}
	}
}

func (w *watcher) Stop() error {
	w.once.Do(func() {
		close(w.stop)
		w.cancel()
	})
	return nil
}

// instancesToServiceInstances converts Eureka instances into registry instances,
// keeping only UP instances.
func instancesToServiceInstances(name, protocol string, instances []eurekaInstance) []*registry.ServiceInstance {
	if protocol == "" {
		protocol = "grpc"
	}
	out := make([]*registry.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Status != "UP" {
			continue
		}
		host := inst.HostName
		if host == "" {
			host = inst.IPAddr
		}
		endpoint := protocol + "://" + net.JoinHostPort(host, strconv.Itoa(inst.Port.Value))
		out = append(out, &registry.ServiceInstance{
			ID:        inst.InstanceID,
			Name:      name,
			Metadata:  inst.Metadata,
			Endpoints: []string{endpoint},
		})
	}
	return out
}

// fingerprint produces a stable string describing the current instance set for
// change detection.
func fingerprint(instances []eurekaInstance) string {
	if len(instances) == 0 {
		return ""
	}
	var b []byte
	for _, inst := range instances {
		b = append(b, inst.InstanceID...)
		b = append(b, ':')
		b = append(b, inst.Status...)
		b = append(b, ';')
	}
	return string(b)
}
