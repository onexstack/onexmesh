// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// registrar implements registry.Registrar against etcd. It binds each instance
// to a lease so the instance is automatically removed when the lease expires.
type registrar struct {
	client *clientv3.Client
	opts   Options

	mu         sync.Mutex
	leaseID    clientv3.LeaseID
	cancel     context.CancelFunc
	registered bool
	inst       *registry.ServiceInstance
}

// NewRegistrar creates an Etcd Registrar.
func NewRegistrar(opts Options) (registry.Registrar, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   opts.Endpoints,
		DialTimeout: opts.DialTimeout,
		Username:    opts.Username,
		Password:    opts.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd: create client: %w", err)
	}
	return &registrar{client: client, opts: opts}, nil
}

func (r *registrar) Register(ctx context.Context, inst *registry.ServiceInstance) error {
	if inst == nil {
		return fmt.Errorf("etcd: nil instance")
	}

	// Assign a stable ID once so re-registration after a lease loss reuses the
	// same key instead of creating a new one.
	if inst.ID == "" {
		inst.ID = fmt.Sprintf("%s-%d", inst.Name, time.Now().UnixNano())
	}

	r.mu.Lock()
	r.inst = inst
	r.mu.Unlock()

	return r.registerOnce(ctx, inst)
}

// registerOnce grants a lease, writes the instance key bound to it, starts
// keepalive, and spawns a monitor that re-registers if the lease is lost.
func (r *registrar) registerOnce(ctx context.Context, inst *registry.ServiceInstance) error {
	lease, err := r.client.Grant(ctx, r.opts.TTL)
	if err != nil {
		return fmt.Errorf("etcd: grant lease: %w", err)
	}

	value, err := json.Marshal(inst)
	if err != nil {
		return fmt.Errorf("etcd: marshal instance: %w", err)
	}

	key := r.key(inst)
	if _, err := r.client.Put(ctx, key, string(value), clientv3.WithLease(lease.ID)); err != nil {
		return fmt.Errorf("etcd: put instance: %w", err)
	}

	keepCtx, cancel := context.WithCancel(context.Background())
	keepCh, err := r.client.KeepAlive(keepCtx, lease.ID)
	if err != nil {
		cancel()
		return fmt.Errorf("etcd: keepalive: %w", err)
	}

	r.mu.Lock()
	// Cancel the previous keepalive context (if any) before replacing it, so a
	// re-registration after a lease loss does not leak the old context.
	if r.cancel != nil {
		r.cancel()
	}
	r.leaseID = lease.ID
	r.cancel = cancel
	r.registered = true
	r.mu.Unlock()

	go r.monitorLease(keepCh)
	return nil
}

// monitorLease drains keepalive responses. When the channel closes it decides
// whether the close was an explicit Deregister (registered=false) or an
// unexpected lease loss, and re-registers in the latter case.
func (r *registrar) monitorLease(keepCh <-chan *clientv3.LeaseKeepAliveResponse) {
	for range keepCh {
	}

	r.mu.Lock()
	inst := r.inst
	registered := r.registered
	r.mu.Unlock()
	if !registered || inst == nil {
		return
	}

	slog.Warn("etcd lease lost, re-registering", "name", inst.Name, "id", inst.ID)
	backoff := time.Second
	for {
		r.mu.Lock()
		if !r.registered {
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()

		if err := r.registerOnce(context.Background(), inst); err == nil {
			return
		}

		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (r *registrar) Deregister(ctx context.Context, inst *registry.ServiceInstance) error {
	r.mu.Lock()
	if !r.registered {
		r.mu.Unlock()
		return nil
	}
	leaseID := r.leaseID
	cancel := r.cancel
	r.registered = false
	r.mu.Unlock()

	// Stop keepalive, then revoke the lease which removes the key.
	cancel()
	if _, err := r.client.Revoke(ctx, leaseID); err != nil {
		return fmt.Errorf("etcd: revoke lease: %w", err)
	}
	// Release the client connection once the instance is fully removed.
	return r.client.Close()
}

// key builds the etcd key for an instance.
func (r *registrar) key(inst *registry.ServiceInstance) string {
	ns := r.opts.Namespace
	if ns == "" {
		ns = "default"
	}
	id := inst.ID
	if id == "" {
		id = fmt.Sprintf("%s-%d", inst.Name, time.Now().UnixNano())
	}
	return path.Join("/onexmesh", ns, inst.Name, id)
}
