// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package informer

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// versionInterfaceView is the view for informers/<group>/<version>/interface.go.
type versionInterfaceView struct {
	Spec    *spec.Spec
	Version *spec.VersionSpec
}

// groupInterfaceView is the view for informers/<group>/interface.go.
type groupInterfaceView struct {
	Spec  *spec.Spec
	Group *spec.GroupSpec
}

// GenerateVersionInterface renders informers/<group>/<version>/interface.go.
func GenerateVersionInterface(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return templates.Render("informer_version_interface", &versionInterfaceView{Spec: s, Version: vs})
}

// GenerateGroupInterface renders informers/<group>/interface.go.
func GenerateGroupInterface(s *spec.Spec, g *spec.GroupSpec) string {
	return templates.Render("informer_group_interface", &groupInterfaceView{Spec: s, Group: g})
}
