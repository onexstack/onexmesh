// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kubernetes

import (
	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// backend implements registry.Backend, contributing the kubernetes flags and
// constructing registrar/discovery from typed Options.
type backend struct {
	opts Options
}

func (b *backend) Name() string { return "kubernetes" }

func (b *backend) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&b.opts.Namespace, prefix+".namespace", b.opts.Namespace, "Kubernetes default namespace for service lookups.")
	fs.StringVar(&b.opts.PortName, prefix+".port-name", b.opts.PortName, "Kubernetes endpoint port name.")
	fs.StringVar(&b.opts.Kubeconfig, prefix+".kubeconfig", b.opts.Kubeconfig, "Path to kubeconfig; empty uses in-cluster.")
}

func (b *backend) Decode(raw map[string]any) error {
	return registry.DecodeOptions(raw, &b.opts)
}

// NewRegistrar ignores host/port/protocol: Kubernetes registration is a no-op
// because Pod membership is declared via a Service selector.
func (b *backend) NewRegistrar(_ string, _ int, _ string) (registry.Registrar, error) {
	return NewRegistrar(b.opts)
}

func (b *backend) NewDiscovery() (registry.Discovery, error) {
	return NewDiscovery(b.opts)
}

func init() {
	registry.RegisterBackend("kubernetes", func() registry.Backend {
		return &backend{opts: Options{Namespace: "default"}}
	})
}
