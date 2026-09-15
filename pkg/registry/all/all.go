// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package all blank-imports every built-in registry backend so their init()
// functions self-register a registry.Backend (and the legacy registrar/discovery
// factories). A service binary blank-imports this package so all backends are
// available out of the box; a binary that wants to trim its dependency graph can
// skip it and blank-import only the backends it needs.
package all

import (
	_ "github.com/onexstack/onexmesh/pkg/registry/consul"
	_ "github.com/onexstack/onexmesh/pkg/registry/etcd"
	_ "github.com/onexstack/onexmesh/pkg/registry/eureka"
	_ "github.com/onexstack/onexmesh/pkg/registry/kubernetes"
	_ "github.com/onexstack/onexmesh/pkg/registry/nacos"
	_ "github.com/onexstack/onexmesh/pkg/registry/polaris"
)
