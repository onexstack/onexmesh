// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/config"
	_ "github.com/onexstack/onexmesh/pkg/config/source/file"
	_ "github.com/onexstack/onexmesh/pkg/config/source/polaris"
)

var _ IOptions = (*ConfigOptions)(nil)

// ConfigOptions selects and configures the configuration center backend.
type ConfigOptions struct {
	Type      string // none, file, polaris
	FilePaths []string

	// Polaris config center.
	PolarisAddresses []string
	PolarisNamespace string
	PolarisFileGroup string
	PolarisFileName  string
}

// NewConfigOptions returns default config options.
func NewConfigOptions() *ConfigOptions {
	return &ConfigOptions{
		Type:             "none",
		PolarisAddresses: []string{"127.0.0.1:8093"},
		PolarisNamespace: "default",
		PolarisFileGroup: "default",
		PolarisFileName:  "config.yaml",
	}
}

func (o *ConfigOptions) Validate() []error {
	if o.Type == "none" || config.RegisteredSource(o.Type) {
		return nil
	}
	return []error{fmt.Errorf("invalid config type %q", o.Type)}
}

func (o *ConfigOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&o.Type, prefix+".type", o.Type, "Config type: none, file, polaris.")
	fs.StringSliceVar(&o.FilePaths, prefix+".file-paths", o.FilePaths, "Config file paths.")
	fs.StringSliceVar(&o.PolarisAddresses, prefix+".polaris-addresses", o.PolarisAddresses, "Polaris config center addresses.")
	fs.StringVar(&o.PolarisNamespace, prefix+".polaris-namespace", o.PolarisNamespace, "Polaris config namespace.")
	fs.StringVar(&o.PolarisFileGroup, prefix+".polaris-file-group", o.PolarisFileGroup, "Polaris config file group.")
	fs.StringVar(&o.PolarisFileName, prefix+".polaris-file-name", o.PolarisFileName, "Polaris config file name.")
}
