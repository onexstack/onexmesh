// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package consul

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// backend implements registry.Backend, contributing the consul flags and
// constructing registrar/discovery from typed Options.
type backend struct {
	opts Options
}

func (b *backend) Name() string { return "consul" }

func (b *backend) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&b.opts.Addr, prefix+".addr", b.opts.Addr, "Consul agent address.")
	fs.StringVar(&b.opts.Scheme, prefix+".scheme", b.opts.Scheme, "Consul agent URI scheme (http/https).")
	fs.StringVar(&b.opts.Token, prefix+".token", b.opts.Token, "Consul ACL token.")
	fs.StringVar(&b.opts.Datacenter, prefix+".datacenter", b.opts.Datacenter, "Consul datacenter.")
	fs.StringVar(&b.opts.Namespace, prefix+".namespace", b.opts.Namespace, "Consul Enterprise namespace.")
	fs.DurationVar(&b.opts.TTL, prefix+".ttl", b.opts.TTL, "Consul health check TTL.")
}

func (b *backend) Decode(raw map[string]any) error {
	return registry.DecodeOptions(raw, &b.opts)
}

func (b *backend) NewRegistrar(host string, port int, protocol string) (registry.Registrar, error) {
	b.opts.Host = host
	b.opts.Port = port
	b.opts.Protocol = protocol
	return NewRegistrar(b.opts)
}

func (b *backend) NewDiscovery() (registry.Discovery, error) {
	return NewDiscovery(b.opts)
}

func init() {
	registry.RegisterBackend("consul", func() registry.Backend {
		return &backend{opts: Options{
			Addr:   "127.0.0.1:8500",
			Scheme: "http",
			TTL:    15 * time.Second,
		}}
	})
}
