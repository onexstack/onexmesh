// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package fake

import (
	"fmt"
	"strings"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/typed"
)

// GenerateGroup renders a fake group client (fake_apps_client.go).
func GenerateGroup(tc *typed.TypeContext) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("package fake\n\n")
	b.WriteString(groupImports(tc))

	structName := fakeGroupName(tc)
	typedPkg := typedPkgAlias(tc)

	fmt.Fprintf(&b, "// %s implements %s.\n", structName, tc.GroupInterface)
	fmt.Fprintf(&b, "type %s struct {\n", structName)
	b.WriteString("\t*testing.Fake\n")
	b.WriteString("}\n\n")

	for _, rs := range tc.Version.Resources {
		plural := pluralGoName(rs)
		if rs.Namespaced {
			fmt.Fprintf(&b, "func (c *%s) %s(namespace string) %s.%sInterface {\n", structName, plural, typedPkg, rs.Kind)
			fmt.Fprintf(&b, "\treturn newFake%s(c, namespace)\n", plural)
		} else {
			fmt.Fprintf(&b, "func (c *%s) %s() %s.%sInterface {\n", structName, plural, typedPkg, rs.Kind)
			fmt.Fprintf(&b, "\treturn newFake%s(c)\n", plural)
		}
		b.WriteString("}\n\n")
	}

	b.WriteString("func (c *" + structName + ") RESTClient() rest.Interface {\n")
	b.WriteString("\tvar ret *rest.RESTClient\n")
	b.WriteString("\treturn ret\n")
	b.WriteString("}\n")

	return b.String()
}

func groupImports(tc *typed.TypeContext) string {
	var b strings.Builder
	b.WriteString("import (\n")
	fmt.Fprintf(&b, "\t%s %q\n", typedPkgAlias(tc), typedPkgPath(tc))
	b.WriteString("\trest \"k8s.io/client-go/rest\"\n")
	b.WriteString("\ttesting \"k8s.io/client-go/testing\"\n")
	b.WriteString(")\n\n")
	return b.String()
}

// typedPkgAlias returns the import alias of the parent typed package, e.g.
// "typedappsv1".
func typedPkgAlias(tc *typed.TypeContext) string {
	return "typed" + typedAlias(tc.Group, tc.Version)
}

// typedPkgPath returns the import path of the parent typed package.
func typedPkgPath(tc *typed.TypeContext) string {
	return tc.Spec.ClientsetGoPackage + "/typed/" + groupPkgName(tc.Group) + "/" + tc.Version.Version
}

// fakeGroupName returns the fake group client type name, e.g. "FakeAppsV1".
func fakeGroupName(tc *typed.TypeContext) string {
	return "Fake" + tc.Group.GroupGoName + tc.VersionGoName
}
