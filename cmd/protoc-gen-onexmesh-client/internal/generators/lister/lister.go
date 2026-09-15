// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package lister generates listers/<group>/<version>/<kind>.go and the
// expansion interfaces, in the client-go style (backed by
// k8s.io/client-go/listers.ResourceIndexer).
package lister

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// listerView is the view for a single resource lister file.
type listerView struct {
	Spec     *spec.Spec
	Group    *spec.GroupSpec
	Version  *spec.VersionSpec
	Resource *spec.ResourceSpec
}

// GenerateLister renders a single resource lister file.
func GenerateLister(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec, rs *spec.ResourceSpec) string {
	return templates.Render("lister", &listerView{Spec: s, Group: g, Version: vs, Resource: rs})
}

// GenerateExpansion renders the expansion_generated.go for a group/version.
func GenerateExpansion(vs *spec.VersionSpec) string {
	return templates.Render("lister_expansion", vs)
}
