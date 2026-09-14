// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package rest

// This file blank-imports the built-in selector strategies so their init()
// functions self-register with the selector package. Without it, a binary that
// only imports this package (and never pkg/client/balancer) would fail the
// selector.GetSelector("round_robin") lookup used by NewForMeshConfig.

import (
	_ "github.com/onexstack/onexmesh/pkg/selector/p2c"
	_ "github.com/onexstack/onexmesh/pkg/selector/random"
	_ "github.com/onexstack/onexmesh/pkg/selector/roundrobin"
	_ "github.com/onexstack/onexmesh/pkg/selector/weighted"
)
