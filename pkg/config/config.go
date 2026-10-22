// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package config provides a layered configuration abstraction inspired by
// go-micro: Source (data origin) -> Loader (decode + merge) -> Reader
// (random access) -> Value (typed access). Backends such as local files and
// the Polaris config center implement Source.
package config

import "time"

// KeyValue is a raw configuration key/value returned by a Source.
type KeyValue struct {
	Key    string
	Value  []byte
	Format string // "json", "yaml", "toml"; empty means auto-detect.
}

// Source reads raw configuration from a backend (file, remote config center,
// environment, ...).
type Source interface {
	// Load returns the current raw key/values.
	Load() ([]*KeyValue, error)
	// Watch returns a Watcher that emits raw changes.
	Watch() (Watcher, error)
}

// Watcher streams raw configuration changes.
type Watcher interface {
	Next() ([]*KeyValue, error)
	Stop() error
}

// Loader decodes and merges raw KeyValues into a reader.
type Loader interface {
	Load(sources ...Source) error
}

// Reader provides random access to merged configuration.
type Reader interface {
	// Merge decodes and merges raw key/values.
	Merge(kvs ...*KeyValue) error
	// Value returns the value at a dot-separated path, e.g. "a.b.c".
	Value(path string) Value
	// Source returns the merged configuration as JSON bytes.
	Source() ([]byte, error)
	// Map returns the merged configuration as a nested map.
	//
	// It exists alongside Source because JSON has a single number type: a value
	// written as an integer comes back from Source as a float64 and then needs
	// weakly-typed decoding to land in an int field. A caller that is going to
	// decode the result itself, or hand it to something that will, should take
	// the map and keep the types the source declared.
	Map() map[string]any
}

// Value provides typed access to a configuration value.
type Value interface {
	String() string
	Int() int
	Int64() int64
	Float64() float64
	Bool() bool
	Duration() time.Duration
	Bytes() []byte
	// Scan unmarshals the value into v.
	Scan(v interface{}) error
}

// Config is the top-level entry point. An App typically owns one Config.
type Config interface {
	// Load loads from the configured sources.
	Load() error
	// Watch invokes fn whenever the configuration changes.
	Watch(fn func(Config)) error
	// Close releases resources.
	Close() error
	Value(path string) Value
	Scan(v interface{}) error
	Reader() Reader
}
