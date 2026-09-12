// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/registry/etcd"
	"github.com/onexstack/onexmesh/pkg/registry/polaris"
)

func newRegistryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Query the service registry",
	}
	cmd.AddCommand(newRegistryListCommand())
	return cmd
}

func newRegistryListCommand() *cobra.Command {
	var (
		regType   string
		addr      string
		namespace string
	)
	cmd := &cobra.Command{
		Use:   "list SERVICE",
		Short: "List instances of a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, opts, err := registryOpts(regType, addr, namespace)
			if err != nil {
				return err
			}
			d, err := registry.CreateDiscovery(name, opts)
			if err != nil {
				return err
			}
			instances, err := d.GetService(context.Background(), args[0])
			if err != nil {
				return err
			}
			for _, inst := range instances {
				b, _ := json.Marshal(inst)
				fmt.Println(string(b))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&regType, "type", "polaris", "Registry type: polaris, etcd.")
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8091", "Registry address.")
	cmd.Flags().StringVar(&namespace, "namespace", "default", "Namespace.")
	return cmd
}

func registryOpts(regType, addr, namespace string) (string, any, error) {
	switch regType {
	case "polaris":
		return "polaris", polaris.Options{Addr: addr, Namespace: namespace}, nil
	case "etcd":
		return "etcd", etcd.Options{Endpoints: []string{addr}, Namespace: namespace}, nil
	default:
		return "", nil, fmt.Errorf("unsupported registry type %q", regType)
	}
}
