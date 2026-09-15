// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package register generates zz_generated.register.go, which registers the
// resource types of a group/version into a runtime.Scheme (mirroring the
// output of k8s.io/code-generator's register-gen).
package register

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// Generate renders the full zz_generated.register.go for a group/version.
func Generate(vs *spec.VersionSpec) string {
	return templates.Render("register", vs)
}
