// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package typed generates the clientset, per-group client and per-resource
// typed client files in the client-go style (gentype-backed).
package typed

import (
	"strings"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// TypeContext carries the naming context needed to render a typed client file.
type TypeContext struct {
	Spec           *spec.Spec
	Group          *spec.GroupSpec
	Version        *spec.VersionSpec
	VersionGoName  string // e.g. "V1"
	GroupClient    string // e.g. "AppsV1Client"
	GroupInterface string // e.g. "AppsV1Interface"
	APIAlias       string // import alias of the API types package, e.g. "appsv1"
}

// NewTypeContext builds the naming context for a group/version.
func NewTypeContext(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) *TypeContext {
	return &TypeContext{
		Spec:           s,
		Group:          g,
		Version:        vs,
		VersionGoName:  upperFirst(vs.Version),
		GroupClient:    g.GroupGoName + upperFirst(vs.Version) + "Client",
		GroupInterface: g.GroupGoName + upperFirst(vs.Version) + "Interface",
		APIAlias:       strings.ToLower(g.GroupGoName) + vs.Version,
	}
}

// typeView pairs a TypeContext with the resource being rendered.
type typeView struct {
	*TypeContext
	Resource *spec.ResourceSpec
}

// GenerateType renders a per-resource typed client file (e.g. deployment.go).
func GenerateType(tc *TypeContext, rs *spec.ResourceSpec) string {
	return templates.Render("typed_type", &typeView{TypeContext: tc, Resource: rs})
}

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
