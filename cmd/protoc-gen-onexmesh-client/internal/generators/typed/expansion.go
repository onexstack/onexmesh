// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package typed

import (
	"fmt"
)

// GenerateExpansion renders the generated_expansion.go holding empty expansion
// interfaces for each resource.
func GenerateExpansion(tc *TypeContext) string {
	var b []byte
	b = append(b, header...)
	b = append(b, fmt.Sprintf("package %s\n\n", typedPackageName(tc))...)
	for _, rs := range tc.Version.Resources {
		b = append(b, fmt.Sprintf("// %sExpansion allows custom methods to be added to\n", rs.Kind)...)
		b = append(b, fmt.Sprintf("// %sInterface.\n", rs.Kind)...)
		b = append(b, fmt.Sprintf("type %sExpansion interface{}\n\n", rs.Kind)...)
	}
	return string(b)
}
