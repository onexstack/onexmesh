// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package informer

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// informerView is the view for a single resource informer file.
type informerView struct {
	Spec     *spec.Spec
	Group    *spec.GroupSpec
	Version  *spec.VersionSpec
	Resource *spec.ResourceSpec
}

// GenerateInformer renders a single resource informer file.
func GenerateInformer(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec, rs *spec.ResourceSpec) string {
	return templates.Render("informer", &informerView{Spec: s, Group: g, Version: vs, Resource: rs})
}
