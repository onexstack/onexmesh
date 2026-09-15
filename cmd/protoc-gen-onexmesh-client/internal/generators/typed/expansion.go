// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package typed

import (
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/templates"
)

// GenerateExpansion renders the generated_expansion.go holding empty expansion
// interfaces for each resource.
func GenerateExpansion(tc *TypeContext) string {
	return templates.Render("typed_expansion", tc)
}
