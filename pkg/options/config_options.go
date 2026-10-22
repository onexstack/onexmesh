// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/config"
	// Blank-import the config source backends so that `config-source.type:
	// polaris` is accepted because a file says so rather than because some other
	// package happened to pull the backend in.
	_ "github.com/onexstack/onexmesh/pkg/config/source/file"
	_ "github.com/onexstack/onexmesh/pkg/config/source/polaris"
)

var _ IOptions = (*ConfigOptions)(nil)

// ConfigOptions selects a remote configuration source and delegates
// source-specific configuration to the self-describing backends registered via
// config.RegisterBackend. It mirrors RegistryOptions deliberately: the two
// sections are stated the same way, address a backend under the same shape, and
// gain a backend the same way — one new package, no change here.
//
// This section is inert on its own. It becomes active only when an application
// wires the value in as a source of remote configuration; see LoadConfigTree.
type ConfigOptions struct {
	// Type is the source backend name: none, file or polaris.
	Type string `mapstructure:"type"`

	// Strict makes a config source that cannot be reached a startup failure. By
	// default an unreachable source is reported as a warning and the service
	// starts on its local configuration alone, which is what lets one build run
	// both with and without a config center. A *misspelled* type is always fatal
	// (see Validate): fail-open is for a source that is down, not for a name
	// that does not exist, because the latter silently disables the very
	// mechanism this section exists to configure.
	Strict bool `mapstructure:"strict"`

	// Options collects the per-backend settings that have no field of their own,
	// keyed by backend name. It is ",remain", so a config file addresses a
	// backend under exactly its flag name — config-source.polaris.addresses sets
	// the same setting as --config-source.polaris.addresses, with no second
	// spelling to remember:
	//
	//	config-source:
	//	  type: polaris
	//	  strict: false
	//	  polaris:
	//	    addresses: [polaris.infra-devops.svc.cluster.local:8093]
	//	    namespace: edu-onex
	//	    file-group: ox-iam-apiserver
	//	    file-name: ox-iam-apiserver.yaml
	//
	// The values reach the backend's typed Options in Complete.
	Options map[string]any `mapstructure:",remain"`

	// backends holds the backend instances created while AddFlags registered
	// their flags, keyed by backend name. It is populated during AddFlags.
	backends map[string]config.Backend

	// fs and flagPrefix are kept from AddFlags so Complete can tell a setting
	// that came from the file apart from one the user passed explicitly.
	fs         *pflag.FlagSet
	flagPrefix string

	// mu guards source. The source is built once and reused: a watch and the
	// reloads it drives must share one client rather than construct one per
	// change, since each construction opens connections and background tasks.
	mu     sync.Mutex
	source config.Source
}

// NewConfigOptions returns default config options.
func NewConfigOptions() *ConfigOptions {
	return &ConfigOptions{
		Type:     "none",
		backends: make(map[string]config.Backend),
	}
}

// Enabled reports whether a source is selected.
func (o *ConfigOptions) Enabled() bool {
	return o.Type != "" && o.Type != "none"
}

func (o *ConfigOptions) Validate() []error {
	var errs []error
	if o.Enabled() && !config.RegisteredBackend(o.Type) {
		errs = append(errs, fmt.Errorf("invalid config-source type %q; known types: %s",
			o.Type, strings.Join(config.BackendNames(), ", ")))
	}
	// A backend named in the file but never registered would otherwise be
	// dropped without a word, so it is reported rather than skipped.
	for name := range o.Options {
		if !config.RegisteredBackend(name) {
			errs = append(errs, fmt.Errorf("config-source.%s: unknown config source; known types: %s",
				name, strings.Join(config.BackendNames(), ", ")))
		}
	}
	return errs
}

// AddFlags registers the shared type and strict flags, then lets every
// registered backend contribute its own flags under a nested prefix
// (--config-source.<name>.*). All backends are registered up front so the flag
// set does not depend on which type is selected.
func (o *ConfigOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	o.fs = fs
	o.flagPrefix = prefix
	fs.StringVar(&o.Type, prefix+".type", o.Type, "Config source type: none, file, polaris.")
	fs.BoolVar(&o.Strict, prefix+".strict", o.Strict,
		"Fail startup when the config source cannot be reached, instead of warning and using the local configuration.")
	for _, name := range config.BackendNames() {
		b, err := config.NewBackend(name)
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
// It exists for the same reason RegistryOptions.Complete does: AddFlags alone
// leaves a backend configurable only from the command line, since what a file
// carries under config-source.<name>.* has nowhere to go — a backend's typed
// Options is by design not a field of this struct. Before this ran, an address
// written into a config file was silently ignored: the file parsed, the service
// started, and the setting had no effect.
func (o *ConfigOptions) Complete() error {
	for name, raw := range o.Options {
		b, err := o.backendNamed(name)
		if err != nil {
			return fmt.Errorf("config-source.%s: %w", name, err)
		}
		if b == nil {
			// Validate reports an unknown backend; skipping it here keeps one
			// mistake from producing two messages.
			continue
		}
		sub, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("config-source.%s: expected a mapping of settings, got %T", name, raw)
		}
		if err := b.Decode(o.withoutExplicitFlags(name, sub)); err != nil {
			return fmt.Errorf("config-source.%s: %w", name, err)
		}
	}
	return nil
}

// withoutExplicitFlags drops the settings the user also passed on the command
// line, keeping precedence flag > config > default as it is everywhere else in
// the application. The flag binding has already written the explicit value into
// the backend, and applying the file over it would silently reverse the two.
func (o *ConfigOptions) withoutExplicitFlags(name string, raw map[string]any) map[string]any {
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
func (o *ConfigOptions) backendNamed(name string) (config.Backend, error) {
	if b, ok := o.backends[name]; ok {
		return b, nil
	}
	if !config.RegisteredBackend(name) {
		return nil, nil
	}
	b, err := config.NewBackend(name)
	if err != nil {
		return nil, err
	}
	if o.backends == nil {
		o.backends = make(map[string]config.Backend)
	}
	o.backends[name] = b
	return b, nil
}

// backend returns the Backend for the configured type, or nil when no source is
// selected. It falls back to constructing a fresh backend when AddFlags was not
// called (programmatic construction).
func (o *ConfigOptions) backend() (config.Backend, error) {
	if !o.Enabled() {
		return nil, nil
	}
	return o.backendNamed(o.Type)
}

// ---------------------------------------------------------------------------
// Application-facing contract
//
// LoadConfigTree, WatchConfigTree and CloseConfigSource are what an application
// framework calls to fold a remote configuration source into its own config
// pipeline. They are shaped so that *ConfigOptions satisfies such an interface
// structurally: nothing in this module imports the framework that declares it,
// and wiring a service is one line.
//
// The two loaders absorb the strict/fail-open decision themselves, because this
// type is the only one that knows what was configured:
//   - strict, or a misconfiguration: return an error, which the framework treats
//     as fatal;
//   - not strict and merely unreachable: log a warning and return (nil, nil),
//     which the framework treats as "no remote configuration".
// ---------------------------------------------------------------------------

// LoadConfigTree loads the merged configuration from the selected source.
//
// It returns (nil, nil) when no source is selected, or when one is selected in
// the default non-strict mode but cannot be reached. The tree is the source's
// own merge of everything it was configured to read; whoever calls this decides
// how it layers over the local file.
func (o *ConfigOptions) LoadConfigTree(ctx context.Context) (map[string]any, error) {
	tree, err := o.loadTree()
	if err != nil {
		if o.Strict {
			return nil, fmt.Errorf("config-source %s: %w", o.Type, err)
		}
		slog.Warn("config source unreachable; continuing with local configuration only",
			"type", o.Type, "err", err)
		return nil, nil
	}
	return tree, nil
}

func (o *ConfigOptions) loadTree() (map[string]any, error) {
	src, err := o.sourceFor()
	if err != nil || src == nil {
		return nil, err
	}

	// Reuse the existing layering rather than merging here: config.New walks the
	// sources through the loader and the reader, which already decode by the
	// KeyValue's declared format and deep-merge nested maps.
	//
	// Reader().Map() rather than Scan: Scan goes through JSON, which has one
	// number type, so every integer in the source would arrive as a float64 and
	// depend on weakly-typed decoding further down to land in an int field.
	cfg := config.New(config.WithSource(src))
	if err := cfg.Load(); err != nil {
		return nil, err
	}
	return cfg.Reader().Map(), nil
}

// WatchConfigTree calls onChange with a freshly loaded tree whenever the source
// changes. It returns as soon as the watch is established; onChange runs on its
// own goroutine until ctx is cancelled.
//
// The callback receives a freshly loaded tree rather than the changed keys,
// because a source can only report what its new content is — not which keys
// vanished. Merging a change incrementally would therefore keep a setting the
// new revision removed, and the running configuration would drift from the one
// the operator can see.
func (o *ConfigOptions) WatchConfigTree(ctx context.Context, onChange func(map[string]any)) error {
	if !o.Enabled() {
		return nil
	}
	src, err := o.sourceFor()
	if err != nil || src == nil {
		if o.Strict {
			return fmt.Errorf("config-source %s: %w", o.Type, err)
		}
		slog.Warn("config source unreachable; configuration changes will not be picked up",
			"type", o.Type, "err", err)
		return nil
	}
	w, err := src.Watch()
	if err != nil {
		if o.Strict {
			return fmt.Errorf("config-source %s: watch: %w", o.Type, err)
		}
		slog.Warn("config source cannot be watched; continuing with the startup configuration",
			"type", o.Type, "err", err)
		return nil
	}

	go func() {
		defer w.Stop()
		// Next blocks until a change arrives or the watcher is stopped, so
		// cancellation has to come from another goroutine. This one exits with
		// the ctx.
		go func() {
			<-ctx.Done()
			_ = w.Stop()
		}()

		for {
			if _, err := w.Next(); err != nil {
				return
			}
			tree, err := o.LoadConfigTree(ctx)
			if err != nil {
				slog.Error("reloading configuration after a change failed; keeping the previous configuration",
					"type", o.Type, "err", err)
				continue
			}
			if tree == nil {
				continue
			}
			onChange(tree)
		}
	}()
	return nil
}

// CloseConfigSource releases the source's resources. Sources that hold a client
// expose Destroy for this; the rest have nothing to release.
func (o *ConfigOptions) CloseConfigSource() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.source == nil {
		return nil
	}
	if d, ok := o.source.(interface{ Destroy() }); ok {
		d.Destroy()
	}
	o.source = nil
	return nil
}

// sourceFor returns the source, building it on first use. It returns (nil, nil)
// when no source is selected.
func (o *ConfigOptions) sourceFor() (config.Source, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.source != nil {
		return o.source, nil
	}
	b, err := o.backend()
	if err != nil || b == nil {
		return nil, err
	}
	src, err := b.NewSource()
	if err != nil {
		return nil, err
	}
	o.source = src
	return src, nil
}
