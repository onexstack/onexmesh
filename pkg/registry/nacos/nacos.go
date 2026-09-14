// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package nacos

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"

	"github.com/onexstack/onexmesh/pkg/registry"
)

func init() {
	registry.RegisterRegistrar("nacos", func(opts any) (registry.Registrar, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("nacos: options must be nacos.Options, got %T", opts)
		}
		return NewRegistrar(o)
	})
	registry.RegisterDiscovery("nacos", func(opts any) (registry.Discovery, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("nacos: options must be nacos.Options, got %T", opts)
		}
		return NewDiscovery(o)
	})
}

// newNamingClient builds a Nacos naming client from Options.
func newNamingClient(opts Options) (naming_client.INamingClient, error) {
	addr := opts.Addr
	if addr == "" {
		addr = "127.0.0.1:8848"
	}
	host, port := splitHostPort(addr)

	serverConfigs := []constant.ServerConfig{{
		IpAddr:      host,
		Port:        port,
		ContextPath: opts.ContextPath,
	}}

	clientConfig := constant.ClientConfig{
		NamespaceId: opts.NamespaceID,
		Username:    opts.Username,
		Password:    opts.Password,
		TimeoutMs:   10000,
		LogLevel:    "warn",
	}

	client, err := clients.CreateNamingClient(map[string]interface{}{
		constant.KEY_SERVER_CONFIGS: serverConfigs,
		constant.KEY_CLIENT_CONFIG:  clientConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("nacos: create naming client: %w", err)
	}
	return client, nil
}

// groupName returns the effective group name, defaulting to DEFAULT_GROUP.
func groupName(opts Options) string {
	if opts.GroupName == "" {
		return constant.DEFAULT_GROUP
	}
	return opts.GroupName
}

// clusterName returns the effective cluster name, defaulting to DEFAULT.
func clusterName(opts Options) string {
	if opts.ClusterName == "" {
		return "DEFAULT"
	}
	return opts.ClusterName
}

// splitHostPort splits a Nacos server "host:port" address into host and a
// uint64 port, defaulting to 8848 when the port is absent or malformed. It uses
// a LastIndex split (not net.SplitHostPort) because it parses a remote server
// address rather than a listen address; this is intentionally distinct from the
// splitHostPort in pkg/app/mesh.go.
func splitHostPort(addr string) (string, uint64) {
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return addr, 8848
	}
	port, err := strconv.ParseUint(addr[idx+1:], 10, 64)
	if err != nil {
		return addr[:idx], 8848
	}
	return addr[:idx], port
}
