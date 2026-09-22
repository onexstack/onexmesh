// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package nacos

import (
	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// backend implements registry.Backend, contributing the nacos flags and
// constructing registrar/discovery from typed Options.
type backend struct {
	opts Options
}

func (b *backend) Name() string { return "nacos" }

func (b *backend) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&b.opts.Addr, prefix+".addr", b.opts.Addr, "Nacos server address.")
	fs.StringVar(&b.opts.ContextPath, prefix+".context-path", b.opts.ContextPath, "Nacos server context path.")
	fs.StringVar(&b.opts.NamespaceID, prefix+".namespace-id", b.opts.NamespaceID, "Nacos namespace ID.")
	fs.StringVar(&b.opts.GroupName, prefix+".group-name", b.opts.GroupName, "Nacos service group name.")
	fs.StringVar(&b.opts.ClusterName, prefix+".cluster-name", b.opts.ClusterName, "Nacos service cluster name.")
	fs.StringVar(&b.opts.Username, prefix+".username", b.opts.Username, "Nacos username.")
	fs.StringVar(&b.opts.Password, prefix+".password", b.opts.Password, "Nacos password.")
	fs.Float64Var(&b.opts.Weight, prefix+".weight", b.opts.Weight, "Nacos instance weight.")
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
	registry.RegisterBackend("nacos", func() registry.Backend {
		return &backend{opts: Options{
			Addr:        "127.0.0.1:8848",
			GroupName:   "DEFAULT_GROUP",
			ClusterName: "DEFAULT",
			Weight:      100,
		}}
	})
}
