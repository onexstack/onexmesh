// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package mesh renders the onexmesh service-discovery client for a gRPC
// service, generated only when the file declares onexmesh.v1.mesh_service.
package mesh

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// Generate renders zz_generated.mesh.go for a single gRPC service.
func Generate(fm *spec.FileMesh) string {
	return templates.Render("mesh", fm)
}

// GenerateHTTPClient renders zz_generated.httpclient.go for a service whose
// methods carry onexmesh.v1.http annotations, providing a typed HTTP client
// with service discovery (mirroring the gRPC mesh client).
func GenerateHTTPClient(fm *spec.FileMesh) string {
	return templates.Render("httpclient", fm)
}
