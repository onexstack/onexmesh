// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package etcd

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// backend implements registry.Backend, contributing the etcd flags and
// constructing registrar/discovery from typed Options. It is the etcd half of
// the self-describing backend contract; see pkg/registry.Backend.
type backend struct {
	opts Options
}

func (b *backend) Name() string { return "etcd" }

func (b *backend) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringSliceVar(&b.opts.Endpoints, prefix+".endpoints", b.opts.Endpoints, "Etcd cluster endpoints.")
	fs.DurationVar(&b.opts.DialTimeout, prefix+".dial-timeout", b.opts.DialTimeout, "Etcd dial timeout.")
	fs.Int64Var(&b.opts.TTL, prefix+".ttl", b.opts.TTL, "Etcd lease TTL in seconds.")
	fs.StringVar(&b.opts.Username, prefix+".username", b.opts.Username, "Etcd username.")
	fs.StringVar(&b.opts.Password, prefix+".password", b.opts.Password, "Etcd password.")
	fs.StringVar(&b.opts.Namespace, prefix+".namespace", b.opts.Namespace, "Etcd key namespace.")
}

// NewRegistrar ignores host/port/protocol: etcd registration carries the
// endpoints in the ServiceInstance, not in Options.
func (b *backend) NewRegistrar(_ string, _ int, _ string) (registry.Registrar, error) {
	return NewRegistrar(b.opts)
}

func (b *backend) NewDiscovery() (registry.Discovery, error) {
	return NewDiscovery(b.opts)
}

func init() {
	registry.RegisterBackend("etcd", func() registry.Backend {
		return &backend{opts: Options{
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: 5 * time.Second,
			TTL:         15,
			Namespace:   "default",
		}}
	})
}
