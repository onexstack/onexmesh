// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/onexstack/onexmesh/pkg/config"
	"github.com/onexstack/onexmesh/pkg/config/source/file"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read configuration",
	}
	cmd.AddCommand(newConfigGetCommand())
	return cmd
}

func newConfigGetCommand() *cobra.Command {
	var (
		path string
		key  string
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Read a config value from a local file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				return fmt.Errorf("--path is required")
			}
			cfg := config.New(config.WithSource(file.New(path)))
			if err := cfg.Load(); err != nil {
				return err
			}
			fmt.Println(cfg.Value(key).String())
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "Config file path.")
	cmd.Flags().StringVar(&key, "key", "", "Dot-separated key to read.")
	return cmd
}
