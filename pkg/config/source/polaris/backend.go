// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package polaris

import (
	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/config"
)

// backend implements config.Backend, contributing the Polaris config center's
// flags and constructing a Source from them.
type backend struct {
	opts Options
}

func (b *backend) Name() string { return "polaris" }

func (b *backend) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringSliceVar(&b.opts.Addresses, prefix+".addresses", b.opts.Addresses,
		"Polaris config center addresses (host:port, config port 8093 — not the discovery port 8091).")
	fs.StringVar(&b.opts.Namespace, prefix+".namespace", b.opts.Namespace, "Polaris config namespace.")
	fs.StringVar(&b.opts.FileGroup, prefix+".file-group", b.opts.FileGroup, "Polaris config file group.")
	fs.StringVar(&b.opts.FileName, prefix+".file-name", b.opts.FileName, "Polaris config file name.")
	fs.StringVar(&b.opts.Format, prefix+".format", b.opts.Format, "Format of the remote file: yaml (default), json or toml.")
	// A token is a credential. It has a flag for completeness, but a deployment
	// should pass it through the environment (OX_<SERVICE>_CONFIG_SOURCE_POLARIS_TOKEN)
	// rather than write it into a file that is committed.
	fs.StringVar(&b.opts.Token, prefix+".token", b.opts.Token, "Polaris access token, if the server requires one.")
}

func (b *backend) Decode(raw map[string]any) error {
	return config.DecodeOptions(raw, &b.opts)
}

func (b *backend) NewSource() (config.Source, error) {
	return New(b.opts)
}

func init() {
	config.RegisterBackend("polaris", func() config.Backend {
		return &backend{opts: Options{
			Addresses: []string{"127.0.0.1:8093"},
			Namespace: "default",
			Format:    "yaml",
		}}
	})
}
