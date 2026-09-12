// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package registry_test

import (
	"testing"

	"github.com/onexstack/onexmesh/pkg/registry"
	_ "github.com/onexstack/onexmesh/pkg/registry/consul"
	_ "github.com/onexstack/onexmesh/pkg/registry/etcd"
	_ "github.com/onexstack/onexmesh/pkg/registry/eureka"
	_ "github.com/onexstack/onexmesh/pkg/registry/kubernetes"
	_ "github.com/onexstack/onexmesh/pkg/registry/nacos"
	_ "github.com/onexstack/onexmesh/pkg/registry/polaris"
)

// TestBackendsRegistered verifies that every built-in backend registers both a
// registrar and discovery factory under its canonical name, so validation and
// creation resolve consistently without a hardcoded whitelist.
func TestBackendsRegistered(t *testing.T) {
	for _, name := range []string{"polaris", "etcd", "kubernetes", "consul", "nacos", "eureka"} {
		if !registry.Registered(name) {
			t.Fatalf("backend %q not registered", name)
		}
	}
}
