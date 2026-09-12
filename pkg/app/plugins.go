// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package app

// This file links every built-in registry backend into the default binary. It
// is the single place that decides which discovery backends are available out
// of the box. A binary that wants to trim its dependency graph can copy this
// file and blank-import only the backends it needs.
import (
	_ "github.com/onexstack/onexmesh/pkg/registry/all"
)
