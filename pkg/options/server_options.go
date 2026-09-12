// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"errors"

	"github.com/spf13/pflag"
)

// ServerOptions is the root options struct combining all leaf options. It
// satisfies the app.FlagSetOptions contract (AddFlags without prefix and
// Validate returning a single error) so it can be passed directly to app.
type ServerOptions struct {
	Mesh       *MeshOptions
	Slog       *SlogOptions
	OTel       *OTelOptions
	Registry   *RegistryOptions
	Config     *ConfigOptions
	Selector   *SelectorOptions
	Resilience *ResilienceOptions
}

// NewServerOptions returns a ServerOptions with all leaf defaults.
func NewServerOptions() *ServerOptions {
	return &ServerOptions{
		Mesh:       NewMeshOptions(),
		Slog:       NewSlogOptions(),
		OTel:       NewOTelOptions(),
		Registry:   NewRegistryOptions(),
		Config:     NewConfigOptions(),
		Selector:   NewSelectorOptions(),
		Resilience: NewResilienceOptions(),
	}
}

// AddFlags registers all leaf flags under their section prefixes.
func (o *ServerOptions) AddFlags(fs *pflag.FlagSet) {
	o.Mesh.AddFlags(fs, "mesh")
	o.Slog.AddFlags(fs, "log")
	o.OTel.AddFlags(fs, "otel")
	o.Registry.AddFlags(fs, "registry")
	o.Config.AddFlags(fs, "config")
	o.Selector.AddFlags(fs, "selector")
	o.Resilience.AddFlags(fs, "resilience")
}

// Validate aggregates all leaf validation errors into a single error.
func (o *ServerOptions) Validate() error {
	var errs []error
	errs = append(errs, o.Mesh.Validate()...)
	errs = append(errs, o.Slog.Validate()...)
	errs = append(errs, o.OTel.Validate()...)
	errs = append(errs, o.Registry.Validate()...)
	errs = append(errs, o.Config.Validate()...)
	errs = append(errs, o.Selector.Validate()...)
	errs = append(errs, o.Resilience.Validate()...)
	return errors.Join(errs...)
}
