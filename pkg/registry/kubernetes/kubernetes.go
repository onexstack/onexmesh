// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kubernetes

import (
	"fmt"

	"github.com/onexstack/onexmesh/pkg/registry"
)

func init() {
	registry.RegisterRegistrar("kubernetes", func(opts any) (registry.Registrar, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("kubernetes: options must be kubernetes.Options, got %T", opts)
		}
		return NewRegistrar(o)
	})
	registry.RegisterDiscovery("kubernetes", func(opts any) (registry.Discovery, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("kubernetes: options must be kubernetes.Options, got %T", opts)
		}
		return NewDiscovery(o)
	})
}
