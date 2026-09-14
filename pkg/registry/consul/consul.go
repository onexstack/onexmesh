// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package consul

import (
	"fmt"
	"time"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/onexstack/onexmesh/pkg/registry"
)

func init() {
	registry.RegisterRegistrar("consul", func(opts any) (registry.Registrar, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("consul: options must be consul.Options, got %T", opts)
		}
		return NewRegistrar(o)
	})
	registry.RegisterDiscovery("consul", func(opts any) (registry.Discovery, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("consul: options must be consul.Options, got %T", opts)
		}
		return NewDiscovery(o)
	})
}

// newClient builds a Consul API client from Options.
func newClient(opts Options) (*consulapi.Client, error) {
	cfg := consulapi.DefaultConfig()
	if opts.Addr != "" {
		cfg.Address = opts.Addr
	}
	if opts.Scheme != "" {
		cfg.Scheme = opts.Scheme
	}
	if opts.Token != "" {
		cfg.Token = opts.Token
	}
	if opts.Datacenter != "" {
		cfg.Datacenter = opts.Datacenter
	}
	if opts.Namespace != "" {
		cfg.Namespace = opts.Namespace
	}

	client, err := consulapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("consul: create client: %w", err)
	}
	return client, nil
}

// ttl returns the effective TTL, defaulting to 15s.
func ttl(opts Options) time.Duration {
	if opts.TTL <= 0 {
		return 15 * time.Second
	}
	return opts.TTL
}
