// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"
	"log/slog"
	"strings"

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

	// Options collects the per-backend settings that have no field of their
	// own, keyed by backend name. It is ",remain", so a config file addresses a
	// backend under exactly its flag name — registry.polaris.addr sets the same
	// setting as --registry.polaris.addr, with no second spelling to remember:
	//
	//	registry:
	//	  type: polaris
	//	  polaris:
	//	    addr: polaris.infra-devops.svc.cluster.local:80
	//	    namespace: edu-onex
	//
	// The values reach the backend's typed Options in Complete.
	Options map[string]any `mapstructure:",remain"`

	// backends holds the backend instances created while AddFlags registered
	// their flags, keyed by backend name. It is populated during AddFlags.
	backends map[string]registry.Backend

	// fs and flagPrefix are kept from AddFlags so Complete can tell a setting
	// that came from the file apart from one the user passed explicitly.
	fs         *pflag.FlagSet
	flagPrefix string
}

// NewRegistryOptions returns default registry options.
func NewRegistryOptions() *RegistryOptions {
	return &RegistryOptions{
		Type:     "none",
		backends: make(map[string]registry.Backend),
	}
}

func (o *RegistryOptions) Validate() []error {
	var errs []error
	if o.Type != "none" && !registry.Registered(o.Type) {
		errs = append(errs, fmt.Errorf("invalid registry type %q", o.Type))
	}
	// A backend named in the file but never registered would otherwise be
	// dropped without a word — the exact silent-ignore this option is shaped to
	// avoid — so it is reported rather than skipped.
	for name := range o.Options {
		if !registry.Registered(name) {
			errs = append(errs, fmt.Errorf(
				"registry.%s: unknown registry backend; known backends: %s",
				name, strings.Join(registry.BackendNames(), ", ")))
		}
	}
	return errs
}

// AddFlags registers the shared registry type flag, then lets every registered
// backend contribute its own flags under a nested prefix (--registry.<name>.*).
// All backends are registered up front so the flag set does not depend on which
// type is selected.
func (o *RegistryOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	o.fs = fs
	o.flagPrefix = prefix
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

// Complete overlays the configuration file's per-backend settings onto the
// backends' typed Options.
//
// It exists because AddFlags alone leaves a backend configurable only from the
// command line: what a file carries under registry.<name>.* has nowhere to go,
// since a backend's typed Options is by design not a field of this struct.
// Before this, a polaris address written into a config file was silently
// ignored — the file parsed, the service started, and the setting had no effect.
func (o *RegistryOptions) Complete() error {
	for name, raw := range o.Options {
		b, err := o.backendNamed(name)
		if err != nil {
			return fmt.Errorf("registry.%s: %w", name, err)
		}
		if b == nil {
			// Validate reports an unknown backend; skipping it here keeps one
			// mistake from producing two messages.
			continue
		}
		sub, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("registry.%s: expected a mapping of settings, got %T", name, raw)
		}
		if err := b.Decode(o.withoutExplicitFlags(name, sub)); err != nil {
			return fmt.Errorf("registry.%s: %w", name, err)
		}
	}

	o.warnIfSettingsGoUnused()
	return nil
}

// warnIfSettingsGoUnused reports a configuration file that carries settings for
// a backend other than the selected one. It is a warning and not an error
// because a file may legitimately hold settings for a backend the operator
// switches to later — but a deployment that set the address and forgot the type
// would otherwise register nowhere and say nothing about why.
func (o *RegistryOptions) warnIfSettingsGoUnused() {
	var unused []string
	for name := range o.Options {
		if name != o.Type {
			unused = append(unused, name)
		}
	}
	if len(unused) == 0 {
		return
	}
	slog.Warn("registry: configuration for a backend that is not selected is ignored",
		"selected-type", o.Type,
		"configured", strings.Join(unused, ", "),
		"effect", "set registry.type to the backend you meant to configure")
}

// withoutExplicitFlags drops the settings the user also passed on the command
// line, keeping precedence flag > config > default as it is everywhere else in
// the application. The flag binding has already written the explicit value into
// the backend, and applying the file over it would silently reverse the two.
func (o *RegistryOptions) withoutExplicitFlags(name string, raw map[string]any) map[string]any {
	if o.fs == nil {
		return raw
	}
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		flagName := o.flagPrefix + "." + name + "." + k
		if f := o.fs.Lookup(flagName); f != nil && f.Changed {
			continue
		}
		out[k] = v
	}
	return out
}

// backendNamed returns the Backend registered under name, constructing one when
// AddFlags was not called (programmatic construction). It returns (nil, nil)
// for a name no backend claims, leaving the report to Validate.
func (o *RegistryOptions) backendNamed(name string) (registry.Backend, error) {
	if b, ok := o.backends[name]; ok {
		return b, nil
	}
	if !registry.Registered(name) {
		return nil, nil
	}
	b, err := registry.NewBackend(name)
	if err != nil {
		return nil, err
	}
	if o.backends == nil {
		o.backends = make(map[string]registry.Backend)
	}
	o.backends[name] = b
	return b, nil
}

// backend returns the Backend for the configured type, or nil when the type is
// "none". It falls back to constructing a fresh backend when AddFlags was not
// called (programmatic construction).
func (o *RegistryOptions) backend() (registry.Backend, error) {
	if o.Type == "none" {
		return nil, nil
	}
	return o.backendNamed(o.Type)
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
