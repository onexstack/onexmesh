// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package file

import (
	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/config"
)

// backend implements config.Backend, contributing the file source's flags and
// constructing a Source from them.
type backend struct {
	opts Options
}

// Options are the file source's settings as they appear in a configuration
// file.
type Options struct {
	// Paths are the files to read, in order. Later files override earlier ones.
	Paths []string `mapstructure:"paths"`
}

func (b *backend) Name() string { return "file" }

func (b *backend) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringSliceVar(&b.opts.Paths, prefix+".paths", b.opts.Paths, "Config file paths to load, in order.")
}

func (b *backend) Decode(raw map[string]any) error {
	return config.DecodeOptions(raw, &b.opts)
}

func (b *backend) NewSource() (config.Source, error) {
	return New(b.opts.Paths...), nil
}

func init() {
	config.RegisterBackend("file", func() config.Backend {
		return &backend{}
	})
}
