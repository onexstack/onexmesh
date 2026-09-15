// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package meta generates the zz_generated.meta.go adapter that makes a
// generated proto message satisfy runtime.Object and metav1.Object, the type
// constraints required by k8s.io/client-go/gentype.
package meta

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// Generate renders the full zz_generated.meta.go for a group/version.
func Generate(vs *spec.VersionSpec) string {
	return templates.Render("meta", vs)
}
