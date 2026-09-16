// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/registry"
)

var _ IOptions = (*RegistryOptions)(nil)

// RegistryOptions selects the registry backend and delegates backend-specific
// configuration to the self-describing backends registered via
// registry.RegisterBackend. This keeps the options layer open to new backends
// without a per-backend field list or switch: adding a backend is one new
// package plus a blank import in pkg/registry/all.
type RegistryOptions struct {
	// Type is the registry backend name: none, polaris, etcd, kubernetes,
	// consul, nacos or eureka.
	Type string `mapstructure:"type"`

	// backends holds the backend instances created while AddFlags registered
	// their flags, keyed by backend name. It is populated during AddFlags.
	backends map[string]registry.Backend
}

// NewRegistryOptions returns default registry options.
func NewRegistryOptions() *RegistryOptions {
	return &RegistryOptions{
		Type:     "none",
		backends: make(map[string]registry.Backend),
	}
}

func (o *RegistryOptions) Validate() []error {
	if o.Type == "none" || registry.Registered(o.Type) {
		return nil
	}
	return []error{fmt.Errorf("invalid registry type %q", o.Type)}
}

// AddFlags registers the shared registry type flag, then lets every registered
// backend contribute its own flags under a nested prefix (--registry.<name>.*).
// All backends are registered up front so the flag set does not depend on which
// type is selected.
func (o *RegistryOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&o.Type, prefix+".type", o.Type, "Registry type: none, polaris, etcd, kubernetes, consul, nacos, eureka.")
	for _, name := range registry.BackendNames() {
		b, err := registry.NewBackend(name)
		if err != nil {
			continue
		}
		o.backends[name] = b
		b.AddFlags(fs, prefix+"."+name)
	}
}

// backend returns the Backend for the configured type, or nil when the type is
// "none". It falls back to constructing a fresh backend when AddFlags was not
// called (programmatic construction).
func (o *RegistryOptions) backend() (registry.Backend, error) {
	if o.Type == "none" {
		return nil, nil
	}
	if b, ok := o.backends[o.Type]; ok {
		return b, nil
	}
	b, err := registry.NewBackend(o.Type)
	if err != nil {
		return nil, err
	}
	if o.backends == nil {
		o.backends = make(map[string]registry.Backend)
	}
	o.backends[o.Type] = b
	return b, nil
}

// NewRegistrar creates the server-side registrar for this configuration. It
// returns (nil, nil) when the registry type is "none".
func (o *RegistryOptions) NewRegistrar(host string, port int, protocol string) (registry.Registrar, error) {
	b, err := o.backend()
	if err != nil || b == nil {
		return nil, err
	}
	return b.NewRegistrar(host, port, protocol)
}

// NewDiscovery creates the client-side discovery for this configuration. It
// returns (nil, nil) when the registry type is "none".
func (o *RegistryOptions) NewDiscovery() (registry.Discovery, error) {
	b, err := o.backend()
	if err != nil || b == nil {
		return nil, err
	}
	return b.NewDiscovery()
}
