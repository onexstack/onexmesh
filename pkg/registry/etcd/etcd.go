// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package etcd

import "github.com/onexstack/onexmesh/pkg/registry"

func init() {
	registry.RegisterRegistrar("etcd", func(opts any) (registry.Registrar, error) {
		return NewRegistrar(opts.(Options))
	})
	registry.RegisterDiscovery("etcd", func(opts any) (registry.Discovery, error) {
		return NewDiscovery(opts.(Options))
	})
}
