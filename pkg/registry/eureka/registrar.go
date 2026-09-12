// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package eureka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// registrar implements registry.Registrar against Eureka with a heartbeat
// goroutine that renews the instance lease.
type registrar struct {
	opts Options
	http *http.Client

	mu         sync.Mutex
	instanceID string
	stop       chan struct{}
	once       sync.Once
}

// NewRegistrar creates an Eureka Registrar.
func NewRegistrar(opts Options) (registry.Registrar, error) {
	return &registrar{
		opts: opts,
		http: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

func (r *registrar) Register(_ context.Context, inst *registry.ServiceInstance) error {
	if inst == nil {
		return fmt.Errorf("eureka: nil instance")
	}

	hostName := r.opts.HostName
	if hostName == "" {
		hostName, _ = os.Hostname()
	}
	if hostName == "" {
		hostName = r.opts.Host
	}
	instanceID := inst.ID
	if instanceID == "" {
		instanceID = fmt.Sprintf("%s:%s:%d", hostName, inst.Name, r.opts.Port)
	}

	interval := int(ttl(r.opts).Seconds())
	instance := eurekaInstance{
		HostName:         hostName,
		App:              inst.Name,
		IPAddr:           r.opts.Host,
		VIPAddress:       inst.Name,
		SecureVIPAddress: inst.Name,
		Status:           "UP",
		Port:             eurekaPort{Value: r.opts.Port, Enabled: "true"},
		SecurePort:       eurekaPort{Value: 443, Enabled: "false"},
		DataCenterInfo:   eurekaDataCenterInfo{Class: "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo", Name: "MyOwn"},
		Metadata:         inst.Metadata,
		InstanceID:       instanceID,
		LeaseInfo:        &eurekaLeaseInfo{RenewalIntervalInSecs: interval, DurationInSecs: 3 * interval},
	}

	body, err := json.Marshal(eurekaRegisterRequest{Instance: instance})
	if err != nil {
		return fmt.Errorf("eureka: marshal instance: %w", err)
	}

	url := fmt.Sprintf("%s/apps/%s", serverURL(r.opts), inst.Name)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("eureka: register instance: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("eureka: register instance returned %d", resp.StatusCode)
	}

	r.mu.Lock()
	r.instanceID = instanceID
	r.stop = make(chan struct{})
	r.mu.Unlock()

	go r.heartbeat(inst.Name, instanceID)
	return nil
}

// heartbeat renews the instance lease at half the TTL interval.
func (r *registrar) heartbeat(app, instanceID string) {
	r.mu.Lock()
	stop := r.stop
	r.mu.Unlock()

	interval := ttl(r.opts) / 2
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			url := fmt.Sprintf("%s/apps/%s/%s", serverURL(r.opts), app, instanceID)
			req, err := http.NewRequest(http.MethodPut, url, nil)
			if err != nil {
				continue
			}
			resp, err := r.http.Do(req)
			if err != nil {
				slog.Error("eureka heartbeat failed", "app", app, "instance_id", instanceID, "err", err)
				continue
			}
			resp.Body.Close()
		}
	}
}

func (r *registrar) Deregister(_ context.Context, inst *registry.ServiceInstance) error {
	r.mu.Lock()
	if r.stop == nil {
		r.mu.Unlock()
		return nil
	}
	instanceID := r.instanceID
	r.once.Do(func() { close(r.stop) })
	r.mu.Unlock()

	if inst == nil {
		return nil
	}

	url := fmt.Sprintf("%s/apps/%s/%s", serverURL(r.opts), inst.Name, instanceID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("eureka: deregister instance: %w", err)
	}
	defer resp.Body.Close()
	return nil
}
