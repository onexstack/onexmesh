// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package fake generates the fake clientset and fake typed clients, enabling
// in-memory CRUD for testing (mirroring k8s.io/client-go's fake package).
package fake

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/typed"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// typeView pairs a TypeContext with the resource being rendered.
type typeView struct {
	*typed.TypeContext
	Resource *spec.ResourceSpec
}

// GenerateType renders a fake resource client (fake_deployment.go) backed by
// gentype.FakeClientWithListAndApply.
func GenerateType(tc *typed.TypeContext, rs *spec.ResourceSpec) string {
	return templates.Render("fake_type", &typeView{TypeContext: tc, Resource: rs})
}
