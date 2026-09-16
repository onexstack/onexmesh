// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/pflag"
)

// ServerOptions is the root options struct combining all leaf options. It
// satisfies the app.FlagSetOptions contract (AddFlags without prefix and
// Validate returning a single error) so it can be passed directly to app.
type ServerOptions struct {
	Mesh       *MeshOptions       `mapstructure:"mesh"`
	Slog       *SlogOptions       `mapstructure:"log"`
	OTel       *OTelOptions       `mapstructure:"otel"`
	Registry   *RegistryOptions   `mapstructure:"registry"`
	Config     *ConfigOptions     `mapstructure:"-"`
	Selector   *SelectorOptions   `mapstructure:"selector"`
	Resilience *ResilienceOptions `mapstructure:"resilience"`
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

// Apply initializes the runtime capabilities (plain slog, then OTel trace/metric/
// log) so logging, metrics and tracing all take effect. It is the single entry
// point the composition root calls after options are validated. Plain slog is
// applied first; for non-classic OTel modes initLogs then bridges slog into OTel.
func (o *ServerOptions) Apply() error {
	// Derive OTel service identity from the mesh service name when the user did
	// not set it explicitly.
	if o.OTel.ServiceName == "" || o.OTel.ServiceName == "unknown-service" {
		o.OTel.ServiceName = o.Mesh.ServiceName
	}

	if err := o.Slog.Apply(); err != nil {
		return fmt.Errorf("apply slog: %w", err)
	}
	if err := o.OTel.Apply(); err != nil {
		return fmt.Errorf("apply otel: %w", err)
	}
	return nil
}

// Shutdown releases the OTel providers and any open output files (both OTel and
// plain slog).
func (o *ServerOptions) Shutdown(ctx context.Context) error {
	var errs []error
	if err := o.OTel.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := o.Slog.Shutdown(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
