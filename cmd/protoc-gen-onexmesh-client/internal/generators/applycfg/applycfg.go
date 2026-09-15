// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package applycfg generates per-version apply configuration types
// (<Kind>ApplyConfiguration) that power server-side apply through the gentype
// ClientWithListAndApply / FakeClientWithListAndApply variants.
//
// The generated types live in a dedicated applyconfigurations/<group>/<version>
// package (mirroring k8s.io/client-go/applyconfigurations), so their
// constructor functions (e.g. Deployment(name, namespace)) do not collide with
// the API type of the same name.
package applycfg

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// applycfgView is the view for a per-version apply configurations file.
type applycfgView struct {
	Spec    *spec.Spec
	Group   *spec.GroupSpec
	Version *spec.VersionSpec
}

// Generate renders the apply configuration file for a group/version, writing
// one <Kind>ApplyConfiguration type per resource.
func Generate(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return templates.Render("applycfg", &applycfgView{Spec: s, Group: g, Version: vs})
}
