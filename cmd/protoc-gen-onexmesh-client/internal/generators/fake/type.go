// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package fake generates the fake clientset and fake typed clients, enabling
// in-memory CRUD for testing (mirroring k8s.io/client-go's fake package).
package fake

import (
	"fmt"
	"strings"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/typed"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// GenerateType renders a fake resource client (fake_deployment.go) backed by
// gentype.FakeClientWithListAndApply.
func GenerateType(tc *typed.TypeContext, rs *spec.ResourceSpec) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("package fake\n\n")
	b.WriteString(typeImports(tc))

	plural := pluralGoName(rs)
	structName := "fake" + plural
	typedPkg := typedPkgAlias(tc)
	fakeGroup := fakeGroupName(tc)

	fmt.Fprintf(&b, "// %s implements %sInterface.\n", structName, rs.Kind)
	fmt.Fprintf(&b, "type %s struct {\n", structName)
	fmt.Fprintf(&b, "\t*gentype.FakeClientWithListAndApply[*%s.%s, *%s.%s, *%s.%sApplyConfiguration]\n", tc.APIAlias, rs.Kind, tc.APIAlias, rs.ListKind, applycfgAlias(tc), rs.Kind)
	fmt.Fprintf(&b, "\tFake *%s\n", fakeGroup)
	b.WriteString("}\n\n")

	// Constructor.
	newFuncName := "newFake" + plural
	if rs.Namespaced {
		fmt.Fprintf(&b, "func %s(fake *%s, namespace string) %s.%sInterface {\n", newFuncName, fakeGroup, typedPkg, rs.Kind)
	} else {
		fmt.Fprintf(&b, "func %s(fake *%s) %s.%sInterface {\n", newFuncName, fakeGroup, typedPkg, rs.Kind)
	}
	fmt.Fprintf(&b, "\treturn &%s{\n", structName)
	fmt.Fprintf(&b, "\t\tgentype.NewFakeClientWithListAndApply[*%s.%s, *%s.%s, *%s.%sApplyConfiguration](\n", tc.APIAlias, rs.Kind, tc.APIAlias, rs.ListKind, applycfgAlias(tc), rs.Kind)
	b.WriteString("\t\t\tfake.Fake,\n")
	if rs.Namespaced {
		b.WriteString("\t\t\tnamespace,\n")
	} else {
		b.WriteString("\t\t\t\"\",\n")
	}
	fmt.Fprintf(&b, "\t\t\t%s.SchemeGroupVersion.WithResource(%q),\n", tc.APIAlias, rs.Plural)
	fmt.Fprintf(&b, "\t\t\t%s.SchemeGroupVersion.WithKind(%q),\n", tc.APIAlias, rs.Kind)
	fmt.Fprintf(&b, "\t\t\tfunc() *%s.%s { return &%s.%s{} },\n", tc.APIAlias, rs.Kind, tc.APIAlias, rs.Kind)
	fmt.Fprintf(&b, "\t\t\tfunc() *%s.%s { return &%s.%s{} },\n", tc.APIAlias, rs.ListKind, tc.APIAlias, rs.ListKind)
	fmt.Fprintf(&b, "\t\t\tfunc(dst, src *%s.%s) { dst.Metadata = src.Metadata },\n", tc.APIAlias, rs.ListKind)
	fmt.Fprintf(&b, "\t\t\tfunc(list *%s.%s) []*%s.%s { return list.Items },\n", tc.APIAlias, rs.ListKind, tc.APIAlias, rs.Kind)
	fmt.Fprintf(&b, "\t\t\tfunc(list *%s.%s, items []*%s.%s) { list.Items = items },\n", tc.APIAlias, rs.ListKind, tc.APIAlias, rs.Kind)
	b.WriteString("\t\t),\n")
	b.WriteString("\t\tfake,\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")

	return b.String()
}

func typeImports(tc *typed.TypeContext) string {
	var b strings.Builder
	b.WriteString("import (\n")
	fmt.Fprintf(&b, "\t%s %q\n", tc.APIAlias, tc.Version.GoPackage)
	fmt.Fprintf(&b, "\t%s %q\n", applycfgAlias(tc), applycfgPath(tc))
	fmt.Fprintf(&b, "\t%s %q\n", typedPkgAlias(tc), typedPkgPath(tc))
	b.WriteString("\tgentype \"k8s.io/client-go/gentype\"\n")
	b.WriteString(")\n\n")
	return b.String()
}
