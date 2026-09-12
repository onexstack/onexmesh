// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// onexmeshctl is the CLI tool for the OneXMesh microservice framework.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/onexstack/onexmesh/pkg/version"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "onexmeshctl",
		Short:         "OneXMesh microservice framework CLI",
		Long:          "onexmeshctl manages OneXMesh services, registries and configuration.",
		Version:       version.Get().String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newNewCommand())
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newDoctorCommand())
	cmd.AddCommand(newRegistryCommand())
	cmd.AddCommand(newConfigCommand())
	return cmd
}
