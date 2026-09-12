// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package polaris

import (
	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// backend implements registry.Backend, contributing the polaris flags and
// constructing registrar/discovery from typed Options.
type backend struct {
	opts Options
}

func (b *backend) Name() string { return "polaris" }

func (b *backend) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&b.opts.Addr, prefix+".addr", b.opts.Addr, "Polaris server address.")
	fs.StringVar(&b.opts.Namespace, prefix+".namespace", b.opts.Namespace, "Polaris namespace.")
	fs.StringVar(&b.opts.Token, prefix+".token", b.opts.Token, "Polaris access token.")
	fs.IntVar(&b.opts.TTL, prefix+".ttl", b.opts.TTL, "Polaris heartbeat TTL in seconds.")
}

func (b *backend) NewRegistrar(host string, port int, protocol string) (registry.Registrar, error) {
	b.opts.Host = host
	b.opts.Port = port
	b.opts.Protocol = protocol
	b.opts.Heartbeat = b.opts.TTL > 0
	return NewRegistrar(b.opts)
}

func (b *backend) NewDiscovery() (registry.Discovery, error) {
	return NewDiscovery(b.opts)
}

func init() {
	registry.RegisterBackend("polaris", func() registry.Backend {
		return &backend{opts: Options{
			Addr:      "127.0.0.1:8091",
			Namespace: "default",
			TTL:       5,
		}}
	})
}
