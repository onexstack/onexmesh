// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// discovery implements registry.Discovery against etcd using prefix get/watch.
type discovery struct {
	client *clientv3.Client
	opts   Options
}

// NewDiscovery creates an Etcd Discovery.
func NewDiscovery(opts Options) (registry.Discovery, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   opts.Endpoints,
		DialTimeout: opts.DialTimeout,
		Username:    opts.Username,
		Password:    opts.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd: create client: %w", err)
	}
	return &discovery{client: client, opts: opts}, nil
}

func (d *discovery) GetService(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	resp, err := d.client.Get(ctx, d.prefix(serviceName), clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd: get service: %w", err)
	}

	out := make([]*registry.ServiceInstance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var si registry.ServiceInstance
		if err := json.Unmarshal(kv.Value, &si); err != nil {
			continue
		}
		out = append(out, &si)
	}
	return out, nil
}

func (d *discovery) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	watchCtx, cancel := context.WithCancel(ctx)
	ch := d.client.Watch(watchCtx, d.prefix(serviceName), clientv3.WithPrefix())
	return &watcher{
		client: d.client,
		prefix: d.prefix(serviceName),
		ch:     ch,
		cancel: cancel,
	}, nil
}

func (d *discovery) prefix(serviceName string) string {
	ns := d.opts.Namespace
	if ns == "" {
		ns = "default"
	}
	return path.Join("/onexmesh", ns, serviceName) + "/"
}

// watcher streams instance snapshots for a watched prefix. The first Next does
// a full prefix Get; subsequent calls apply the watch's incremental PUT/DELETE
// events against a local snapshot, avoiding a full Get on every change.
type watcher struct {
	client *clientv3.Client
	prefix string
	ch     clientv3.WatchChan
	cancel context.CancelFunc

	mu      sync.Mutex
	current map[string]*registry.ServiceInstance
	init    bool
}

func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.init {
		w.init = true
		return w.fullSnapshot()
	}

	resp, ok := <-w.ch
	if !ok {
		return nil, errors.New("etcd: watch closed")
	}
	if resp.Err() != nil {
		// On a compacted revision or watch error, resync from a full snapshot so
		// the watcher self-heals instead of surfacing the transient error.
		return w.fullSnapshot()
	}

	for _, ev := range resp.Events {
		key := string(ev.Kv.Key)
		switch ev.Type {
		case clientv3.EventTypePut:
			var si registry.ServiceInstance
			if err := json.Unmarshal(ev.Kv.Value, &si); err != nil {
				continue
			}
			w.current[key] = &si
		case clientv3.EventTypeDelete:
			delete(w.current, key)
		}
	}
	return w.snapshot(), nil
}

// fullSnapshot performs a full prefix Get and rebuilds the local map.
func (w *watcher) fullSnapshot() ([]*registry.ServiceInstance, error) {
	resp, err := w.client.Get(context.Background(), w.prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd: get: %w", err)
	}

	w.current = make(map[string]*registry.ServiceInstance, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var si registry.ServiceInstance
		if err := json.Unmarshal(kv.Value, &si); err != nil {
			continue
		}
		w.current[string(kv.Key)] = &si
	}
	return w.snapshot(), nil
}

// snapshot returns the current instances as a slice.
func (w *watcher) snapshot() []*registry.ServiceInstance {
	out := make([]*registry.ServiceInstance, 0, len(w.current))
	for _, si := range w.current {
		out = append(out, si)
	}
	return out
}

func (w *watcher) Stop() error {
	w.cancel()
	return nil
}
