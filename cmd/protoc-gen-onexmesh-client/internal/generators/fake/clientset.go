// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package fake

import (
	"fmt"
	"strings"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/typed"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// GenerateClientset renders the fake clientset (fake/clientset.go).
func GenerateClientset(s *spec.Spec) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package fake\n\n")
	b.WriteString(clientsetImports(s))

	b.WriteString("// Clientset contains the fake clients for groups.\n")
	b.WriteString("type Clientset struct {\n")
	b.WriteString("\ttesting.Fake\n")
	b.WriteString("\tdiscovery *fakediscovery.FakeDiscovery\n")
	b.WriteString("\ttracker   testing.ObjectTracker\n")
	b.WriteString("}\n\n")

	b.WriteString("// Discovery retrieves the DiscoveryClient.\n")
	b.WriteString("func (c *Clientset) Discovery() discovery.DiscoveryInterface {\n")
	b.WriteString("\treturn c.discovery\n")
	b.WriteString("}\n\n")

	b.WriteString("// Tracker allows access to the object tracker.\n")
	b.WriteString("func (c *Clientset) Tracker() testing.ObjectTracker {\n")
	b.WriteString("\treturn c.tracker\n")
	b.WriteString("}\n\n")

	b.WriteString("// IsWatchListSemanticsUnSupported informs the reflector that this client\n")
	b.WriteString("// doesn't support WatchList semantics.\n//\n")
	b.WriteString("// This is a synthetic method whose sole purpose is to satisfy the optional\n")
	b.WriteString("// interface check performed by the reflector.\n")
	b.WriteString("// Returning true signals that WatchList can NOT be used.\n")
	b.WriteString("// No additional logic is implemented here.\n")
	b.WriteString("func (c *Clientset) IsWatchListSemanticsUnSupported() bool {\n")
	b.WriteString("\treturn true\n")
	b.WriteString("}\n\n")

	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			typedAlias := typedAlias(g, vs)
			fakeAlias := "fake" + typedAlias
			gvName := groupVersionGoName(g, vs)
			fmt.Fprintf(&b, "// %s retrieves the %sClient.\n", gvName, gvName)
			fmt.Fprintf(&b, "func (c *Clientset) %s() %s.%s {\n", gvName, typedAlias, gvName+"Interface")
			fmt.Fprintf(&b, "\treturn &%s.Fake%s{Fake: &c.Fake}\n", fakeAlias, gvName)
			b.WriteString("}\n\n")
		}
	}

	b.WriteString("// NewSimpleClientset returns a Clientset that will respond with the provided objects.\n")
	b.WriteString("func NewSimpleClientset(objects ...runtime.Object) *Clientset {\n")
	b.WriteString("\to := testing.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())\n")
	b.WriteString("\tfor _, obj := range objects {\n")
	b.WriteString("\t\tif err := o.Add(obj); err != nil {\n")
	b.WriteString("\t\t\tpanic(err)\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t}\n\n")
	b.WriteString("\tcs := &Clientset{tracker: o}\n")
	b.WriteString("\tcs.discovery = &fakediscovery.FakeDiscovery{Fake: &cs.Fake}\n")
	b.WriteString("\tcs.AddReactor(\"*\", \"*\", testing.ObjectReaction(o))\n")
	b.WriteString("\tcs.AddWatchReactor(\"*\", func(action testing.Action) (bool, watch.Interface, error) {\n")
	b.WriteString("\t\tgvr := action.GetResource()\n")
	b.WriteString("\t\tns := action.GetNamespace()\n")
	b.WriteString("\t\tw, err := o.Watch(gvr, ns)\n")
	b.WriteString("\t\tif err != nil {\n")
	b.WriteString("\t\t\treturn false, nil, err\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\treturn true, w, nil\n")
	b.WriteString("\t})\n")
	b.WriteString("\treturn cs\n")
	b.WriteString("}\n")

	return b.String()
}

func clientsetImports(s *spec.Spec) string {
	var b strings.Builder
	b.WriteString("import (\n")
	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			fmt.Fprintf(&b, "\t%s %q\n", typedAlias(g, vs), typedPackagePath(s, g, vs))
			fmt.Fprintf(&b, "\t%s %q\n", "fake"+typedAlias(g, vs), typedPackagePath(s, g, vs)+"/fake")
		}
	}
	fmt.Fprintf(&b, "\tscheme %q\n", s.ClientsetGoPackage+"/scheme")
	b.WriteString("\truntime \"k8s.io/apimachinery/pkg/runtime\"\n")
	b.WriteString("\twatch \"k8s.io/apimachinery/pkg/watch\"\n")
	b.WriteString("\tdiscovery \"k8s.io/client-go/discovery\"\n")
	b.WriteString("\tfakediscovery \"k8s.io/client-go/discovery/fake\"\n")
	b.WriteString("\ttesting \"k8s.io/client-go/testing\"\n")
	b.WriteString(")\n\n")
	return b.String()
}

func typedAlias(g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return strings.ToLower(g.GroupGoName) + vs.Version
}

func groupVersionGoName(g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return g.GroupGoName + upperFirst(vs.Version)
}

func typedPackagePath(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return s.ClientsetGoPackage + "/typed/" + groupPkgName(g) + "/" + vs.Version
}

func groupPkgName(g *spec.GroupSpec) string {
	if g.Group == "" {
		return "core"
	}
	return g.Group
}

// applycfgAlias returns the import alias of the apply configurations package.
func applycfgAlias(tc *typed.TypeContext) string {
	return "applyconfigurations" + strings.ToLower(tc.Group.GroupGoName) + tc.Version.Version
}

// applycfgPath returns the import path of the apply configurations package.
func applycfgPath(tc *typed.TypeContext) string {
	return tc.Spec.ClientsetGoPackage + "/applyconfigurations/" + groupPkgName(tc.Group) + "/" + tc.Version.Version
}

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// pluralGoName returns the exported Go name for the resource plural.
func pluralGoName(rs *spec.ResourceSpec) string {
	return upperFirst(rs.Plural)
}

const header = `// Code generated by protoc-gen-onexmesh-client. DO NOT EDIT.

`
