// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package eureka

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// backend implements registry.Backend, contributing the eureka flags and
// constructing registrar/discovery from typed Options.
type backend struct {
	opts Options
}

func (b *backend) Name() string { return "eureka" }

func (b *backend) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&b.opts.ServerURL, prefix+".server-url", b.opts.ServerURL, "Eureka server base URL.")
	fs.StringVar(&b.opts.HostName, prefix+".host-name", b.opts.HostName, "Eureka instance hostname.")
	fs.DurationVar(&b.opts.TTL, prefix+".ttl", b.opts.TTL, "Eureka lease renewal interval.")
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
	registry.RegisterBackend("eureka", func() registry.Backend {
		return &backend{opts: Options{
			ServerURL: "http://127.0.0.1:8761/eureka",
			TTL:       30 * time.Second,
		}}
	})
}
