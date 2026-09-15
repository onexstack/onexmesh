// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package scheme generates the clientset scheme/register.go, which aggregates
// the AddToScheme of every group/version into a single runtime.Scheme.
package scheme

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// Generate renders the clientset scheme/register.go.
func Generate(s *spec.Spec) string {
	return templates.Render("scheme", s)
}
