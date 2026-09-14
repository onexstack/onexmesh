// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package polaris

import (
	"fmt"

	"github.com/onexstack/onexmesh/pkg/registry"
)

func init() {
	registry.RegisterRegistrar("polaris", func(opts any) (registry.Registrar, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("polaris: options must be polaris.Options, got %T", opts)
		}
		return NewRegistrar(o)
	})
	registry.RegisterDiscovery("polaris", func(opts any) (registry.Discovery, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("polaris: options must be polaris.Options, got %T", opts)
		}
		return NewDiscovery(o)
	})
}
