// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"sort"

	"github.com/spf13/pflag"
)

// Backend is a self-describing configuration source. It contributes its own CLI
// flags, holds a typed options value, and constructs its Source from them.
// Concrete backends register a BackendFactory via RegisterBackend in init(); the
// options layer discovers them through this registry rather than importing each
// backend, so adding a source no longer requires touching the options package
// (open/closed principle).
//
// It is the same shape as registry.Backend, one layer up: Source is what reads
// the configuration — and is what a caller who already has its options uses
// directly through CreateSource — while Backend is what lets a *configuration
// file* address a source whose settings the options layer cannot know. A source
// that only ever needs to be constructed in code registers only a SourceFactory.
type Backend interface {
	// Name is the backend identifier, used as the configured type and as the
	// nested flag prefix (e.g. "polaris" -> --config-source.polaris.addresses).
	Name() string
	// AddFlags binds the backend's configuration to flags under prefix. The
	// backend owns a typed Options struct and binds directly to it.
	AddFlags(fs *pflag.FlagSet, prefix string)
	// Decode overlays configuration-file values onto the backend's typed
	// Options. Keys are flag names with the prefix removed, e.g. "addresses" for
	// --config-source.polaris.addresses. Implementations are one call to
	// DecodeOptions.
	Decode(raw map[string]any) error
	// NewSource builds the Source from the parsed options.
	NewSource() (Source, error)
}

// BackendFactory constructs a Backend with its default options.
type BackendFactory func() Backend

// backendFactories maps backend names to their factories. Writes happen only
// from init() during package initialization (single-threaded); reads happen at
// runtime and are safe as long as no concurrent writes occur. It is kept
// separate from sourceFactories, which keys the same names to factories that
// take arbitrary options: a name may legitimately be present in both, and the
// two are consulted by different callers.
var backendFactories = map[string]BackendFactory{}

// RegisterBackend registers a BackendFactory under name. Backend packages call
// this from init().
func RegisterBackend(name string, f BackendFactory) {
	backendFactories[name] = f
}

// NewBackend constructs a fresh Backend by name.
func NewBackend(name string) (Backend, error) {
	f, ok := backendFactories[name]
	if !ok {
		return nil, fmt.Errorf("config backend %q not registered", name)
	}
	return f(), nil
}

// BackendNames returns the sorted names of all registered backends. It is used
// by validation to avoid hardcoding a whitelist, and by AddFlags so the flag set
// does not depend on which type is selected.
func BackendNames() []string {
	names := make([]string, 0, len(backendFactories))
	for name := range backendFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RegisteredBackend reports whether a Backend is registered under name.
func RegisteredBackend(name string) bool {
	_, ok := backendFactories[name]
	return ok
}
