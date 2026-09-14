// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package informer

import (
	"fmt"
	"strings"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// GenerateVersionInterface renders informers/<group>/<version>/interface.go.
func GenerateVersionInterface(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package %s\n\n", vs.Version)
	b.WriteString("import (\n")
	fmt.Fprintf(&b, "\tinternalinterfaces %q\n", informersRoot(s)+"/internalinterfaces")
	b.WriteString(")\n\n")

	b.WriteString("// Interface provides access to all the informers in this group version.\n")
	b.WriteString("type Interface interface {\n")
	for _, rs := range vs.Resources {
		plural := upperFirst(rs.Plural)
		fmt.Fprintf(&b, "\t// %s returns a %sInformer.\n", plural, rs.Kind)
		fmt.Fprintf(&b, "\t%s() %sInformer\n", plural, rs.Kind)
	}
	b.WriteString("}\n\n")

	b.WriteString("type version struct {\n")
	b.WriteString("\tfactory          internalinterfaces.SharedInformerFactory\n")
	b.WriteString("\tnamespace        string\n")
	b.WriteString("\ttweakListOptions internalinterfaces.TweakListOptionsFunc\n")
	b.WriteString("}\n\n")

	b.WriteString("// New returns a new Interface.\n")
	b.WriteString("func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {\n")
	b.WriteString("\treturn &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}\n")
	b.WriteString("}\n\n")

	for _, rs := range vs.Resources {
		plural := upperFirst(rs.Plural)
		lowerKind := lowerFirst(rs.Kind)
		fmt.Fprintf(&b, "// %s returns a %sInformer.\n", plural, rs.Kind)
		fmt.Fprintf(&b, "func (v *version) %s() %sInformer {\n", plural, rs.Kind)
		if rs.Namespaced {
			fmt.Fprintf(&b, "\treturn &%sInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}\n", lowerKind)
		} else {
			fmt.Fprintf(&b, "\treturn &%sInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}\n", lowerKind)
		}
		b.WriteString("}\n\n")
	}

	return b.String()
}

// GenerateGroupInterface renders informers/<group>/interface.go.
func GenerateGroupInterface(s *spec.Spec, g *spec.GroupSpec) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package %s\n\n", groupPkgName(g))
	b.WriteString("import (\n")
	for _, vs := range g.Versions {
		fmt.Fprintf(&b, "\t%s %q\n", vs.Version, versionInformerPath(s, g, vs))
	}
	fmt.Fprintf(&b, "\tinternalinterfaces %q\n", informersRoot(s)+"/internalinterfaces")
	b.WriteString(")\n\n")

	b.WriteString("// Interface provides access to each of this group's versions.\n")
	b.WriteString("type Interface interface {\n")
	for _, vs := range g.Versions {
		fmt.Fprintf(&b, "\t// %s provides access to shared informers for resources in %s.\n", versionGoName(vs), versionGoName(vs))
		fmt.Fprintf(&b, "\t%s() %s.Interface\n", versionGoName(vs), vs.Version)
	}
	b.WriteString("}\n\n")

	b.WriteString("type group struct {\n")
	b.WriteString("\tfactory          internalinterfaces.SharedInformerFactory\n")
	b.WriteString("\tnamespace        string\n")
	b.WriteString("\ttweakListOptions internalinterfaces.TweakListOptionsFunc\n")
	b.WriteString("}\n\n")

	b.WriteString("// New returns a new Interface.\n")
	b.WriteString("func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {\n")
	b.WriteString("\treturn &group{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}\n")
	b.WriteString("}\n\n")

	for _, vs := range g.Versions {
		fmt.Fprintf(&b, "// %s returns a new %s.Interface.\n", versionGoName(vs), vs.Version)
		fmt.Fprintf(&b, "func (g *group) %s() %s.Interface {\n", versionGoName(vs), vs.Version)
		fmt.Fprintf(&b, "\treturn %s.New(g.factory, g.namespace, g.tweakListOptions)\n", vs.Version)
		b.WriteString("}\n\n")
	}

	return b.String()
}
