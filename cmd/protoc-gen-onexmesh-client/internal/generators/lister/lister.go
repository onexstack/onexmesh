// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package lister generates listers/<group>/<version>/<kind>.go and the
// expansion interfaces, in the client-go style (backed by
// k8s.io/client-go/listers.ResourceIndexer).
package lister

import (
	"fmt"
	"strings"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// GenerateLister renders a single resource lister file.
func GenerateLister(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec, rs *spec.ResourceSpec) string {
	apiAlias := apiAlias(g, vs)
	kind := rs.Kind
	pluralGo := upperFirst(rs.Plural)
	lowerKind := lowerFirst(kind)

	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package %s\n\n", vs.Version)
	b.WriteString(listerImports(s, g, vs))

	fmt.Fprintf(&b, "// %sLister helps list %s.\n", kind, pluralGo)
	b.WriteString("// All objects returned here must be treated as read-only.\n")
	fmt.Fprintf(&b, "type %sLister interface {\n", kind)
	b.WriteString("\t// List lists all " + pluralGo + " in the indexer.\n")
	b.WriteString("\t// Objects returned here must be treated as read-only.\n")
	fmt.Fprintf(&b, "\tList(selector labels.Selector) (ret []*%s.%s, err error)\n", apiAlias, kind)
	if rs.Namespaced {
		fmt.Fprintf(&b, "\t// %s returns an object that can list and get %s.\n", pluralGo, pluralGo)
		fmt.Fprintf(&b, "\t%s(namespace string) %sNamespaceLister\n", pluralGo, kind)
	} else {
		fmt.Fprintf(&b, "\t// Get retrieves the %s from the index for a given name.\n", kind)
		b.WriteString("\t// Objects returned here must be treated as read-only.\n")
		fmt.Fprintf(&b, "\tGet(name string) (*%s.%s, error)\n", apiAlias, kind)
	}
	fmt.Fprintf(&b, "\t%sListerExpansion\n", kind)
	b.WriteString("}\n\n")

	// Lister struct.
	fmt.Fprintf(&b, "// %sLister implements the %sLister interface.\n", lowerKind, kind)
	fmt.Fprintf(&b, "type %sLister struct {\n", lowerKind)
	fmt.Fprintf(&b, "\tlisters.ResourceIndexer[*%s.%s]\n", apiAlias, kind)
	b.WriteString("}\n\n")

	// Constructor.
	fmt.Fprintf(&b, "// New%sLister returns a new %sLister.\n", kind, kind)
	fmt.Fprintf(&b, "func New%sLister(indexer cache.Indexer) %sLister {\n", kind, kind)
	fmt.Fprintf(&b, "\treturn &%sLister{listers.New[*%s.%s](indexer, %s.Resource(%q))}\n", lowerKind, apiAlias, kind, apiAlias, rs.Singular)
	b.WriteString("}\n\n")

	if !rs.Namespaced {
		return b.String()
	}

	// Namespaced lister accessor.
	fmt.Fprintf(&b, "// %s returns an object that can list and get %s.\n", pluralGo, pluralGo)
	fmt.Fprintf(&b, "func (s *%sLister) %s(namespace string) %sNamespaceLister {\n", lowerKind, pluralGo, kind)
	fmt.Fprintf(&b, "\treturn %sNamespaceLister{listers.NewNamespaced[*%s.%s](s.ResourceIndexer, namespace)}\n", lowerKind, apiAlias, kind)
	b.WriteString("}\n\n")

	// Namespace lister interface.
	fmt.Fprintf(&b, "// %sNamespaceLister helps list and get %s.\n", kind, pluralGo)
	b.WriteString("// All objects returned here must be treated as read-only.\n")
	fmt.Fprintf(&b, "type %sNamespaceLister interface {\n", kind)
	b.WriteString("\t// List lists all " + pluralGo + " in the indexer for a given namespace.\n")
	b.WriteString("\t// Objects returned here must be treated as read-only.\n")
	fmt.Fprintf(&b, "\tList(selector labels.Selector) (ret []*%s.%s, err error)\n", apiAlias, kind)
	fmt.Fprintf(&b, "\t// Get retrieves the %s from the indexer for a given namespace and name.\n", kind)
	b.WriteString("\t// Objects returned here must be treated as read-only.\n")
	fmt.Fprintf(&b, "\tGet(name string) (*%s.%s, error)\n", apiAlias, kind)
	fmt.Fprintf(&b, "\t%sNamespaceListerExpansion\n", kind)
	b.WriteString("}\n\n")

	// Namespace lister struct.
	fmt.Fprintf(&b, "// %sNamespaceLister implements the %sNamespaceLister\n", lowerKind, kind)
	b.WriteString("// interface.\n")
	fmt.Fprintf(&b, "type %sNamespaceLister struct {\n", lowerKind)
	fmt.Fprintf(&b, "\tlisters.ResourceIndexer[*%s.%s]\n", apiAlias, kind)
	b.WriteString("}\n")

	return b.String()
}

// GenerateExpansion renders the expansion_generated.go for a group/version.
func GenerateExpansion(vs *spec.VersionSpec) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package %s\n\n", vs.Version)
	for _, rs := range vs.Resources {
		fmt.Fprintf(&b, "// %sListerExpansion allows custom methods to be added to\n", rs.Kind)
		fmt.Fprintf(&b, "// %sLister.\n", rs.Kind)
		fmt.Fprintf(&b, "type %sListerExpansion interface{}\n\n", rs.Kind)
		if rs.Namespaced {
			fmt.Fprintf(&b, "// %sNamespaceListerExpansion allows custom methods to be added to\n", rs.Kind)
			fmt.Fprintf(&b, "// %sNamespaceLister.\n", rs.Kind)
			fmt.Fprintf(&b, "type %sNamespaceListerExpansion interface{}\n\n", rs.Kind)
		}
	}
	return b.String()
}

func listerImports(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	var b strings.Builder
	b.WriteString("import (\n")
	fmt.Fprintf(&b, "\t%s %q\n", apiAlias(g, vs), vs.GoPackage)
	b.WriteString("\tlabels \"k8s.io/apimachinery/pkg/labels\"\n")
	b.WriteString("\tlisters \"k8s.io/client-go/listers\"\n")
	b.WriteString("\tcache \"k8s.io/client-go/tools/cache\"\n")
	b.WriteString(")\n\n")
	return b.String()
}

func apiAlias(g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return strings.ToLower(g.GroupGoName) + vs.Version
}

const header = `// Code generated by protoc-gen-onexmesh-client. DO NOT EDIT.

`

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}
