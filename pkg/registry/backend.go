// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
)

// Backend is a self-describing registry backend. It contributes its own CLI
// flags, holds a typed Options value, and constructs its Registrar/Discovery
// from them. Concrete backends register a BackendFactory via RegisterBackend in
// init(); the options layer discovers them through this registry rather than
// importing each backend, so adding a backend no longer requires touching the
// options package (open/closed principle).
type Backend interface {
	// Name is the backend identifier, used as the registry type and as the
	// nested flag prefix (e.g. "etcd" -> --registry.etcd.endpoints).
	Name() string
	// AddFlags binds the backend's configuration to flags under prefix. The
	// backend owns a typed Options struct and binds directly to it.
	AddFlags(fs *pflag.FlagSet, prefix string)
	// Decode overlays configuration-file values onto the backend's Options.
	// Keys are flag names with the prefix removed, e.g. "addr" for
	// --registry.polaris.addr. It is what makes a backend configurable from a
	// YAML file at all: the options layer cannot reach a backend's typed
	// Options by itself, because that is exactly the dependency this registry
	// exists to avoid. Implementations are one call to DecodeOptions.
	Decode(raw map[string]any) error
	// NewRegistrar builds a server-side Registrar from the parsed options.
	// host/port/protocol identify the local instance endpoint; backends that
	// embed them in Options (consul/nacos/eureka/polaris) set them, while
	// others (etcd/kubernetes) ignore them because registration carries the
	// endpoints in the ServiceInstance.
	NewRegistrar(host string, port int, protocol string) (Registrar, error)
	// NewDiscovery builds a client-side Discovery from the parsed options.
	NewDiscovery() (Discovery, error)
}

// DecodeOptions overlays configuration-file values onto a backend's typed
// Options. It is the single implementation behind every backend's Decode, so a
// backend gains file configuration with one line rather than a hand-written
// field mapping per option.
//
// The matcher ignores case and hyphens, so the YAML key "dial-timeout" and the
// Go field DialTimeout are the same option. Without that every multi-word option
// would decode to nothing and leave its default in place — a failure with no
// symptom, since the service starts and merely ignores what it was configured
// with. That is the same shape of mistake this layer exists to remove.
//
// The decoder config mirrors viper's, so a value written in a config file
// behaves here as it does everywhere else in the application: a number or a
// boolean may arrive as a string, and a duration may be written "10s".
func DecodeOptions(raw map[string]any, out any) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           out,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
		MatchName: func(mapKey, fieldName string) bool {
			return normalizeKey(mapKey) == normalizeKey(fieldName)
		},
	})
	if err != nil {
		return err
	}
	return dec.Decode(raw)
}

// normalizeKey folds a key to the form DecodeOptions' matcher compares on:
// case-insensitive and hyphen-blind, so "port-name", "portname" and PortName
// all agree.
func normalizeKey(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "-", ""))
}

// BackendFactory constructs a Backend with its default Options.
type BackendFactory func() Backend

// The backend registry table. Writes happen only from init() during package
// initialization (single-threaded); reads happen at runtime.
var backends = map[string]BackendFactory{}

// RegisterBackend registers a BackendFactory under name. Backend packages call
// this from init().
func RegisterBackend(name string, f BackendFactory) {
	backends[name] = f
}

// NewBackend constructs a fresh Backend by name.
func NewBackend(name string) (Backend, error) {
	f, ok := backends[name]
	if !ok {
		return nil, fmt.Errorf("registry backend %q not registered", name)
	}
	return f(), nil
}

// BackendNames returns the sorted names of all registered backends.
func BackendNames() []string {
	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
