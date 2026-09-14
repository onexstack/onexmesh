// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package typed generates the clientset, per-group client and per-resource
// typed client files in the client-go style (gentype-backed).
package typed

import (
	"fmt"
	"strings"

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

// GenerateType renders a per-resource typed client file (e.g. deployment.go).
func GenerateType(tc *TypeContext, rs *spec.ResourceSpec) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package %s\n\n", typedPackageName(tc))
	b.WriteString(typeImports(tc, rs))

	plural := pluralGoName(rs)

	// Getter interface.
	fmt.Fprintf(&b, "// %sGetter has a method to return a %sInterface.\n", plural, rs.Kind)
	fmt.Fprintf(&b, "// A group's client should implement this interface.\n")
	fmt.Fprintf(&b, "type %sGetter interface {\n", plural)
	if rs.Namespaced {
		fmt.Fprintf(&b, "\t%s(namespace string) %sInterface\n", plural, rs.Kind)
	} else {
		fmt.Fprintf(&b, "\t%s() %sInterface\n", plural, rs.Kind)
	}
	b.WriteString("}\n\n")

	// Resource interface.
	fmt.Fprintf(&b, "// %sInterface has methods to work with %s resources.\n", rs.Kind, rs.Kind)
	fmt.Fprintf(&b, "type %sInterface interface {\n", rs.Kind)
	generateInterfaceMethods(&b, tc, rs)
	b.WriteString("}\n\n")

	// Implementation struct (gentype composition).
	applyAlias := applycfgAlias(tc)
	fmt.Fprintf(&b, "// %s implements %sInterface\n", lowerFirst(plural), rs.Kind)
	fmt.Fprintf(&b, "type %s struct {\n", lowerFirst(plural))
	fmt.Fprintf(&b, "\t*gentype.ClientWithListAndApply[*%s.%s, *%s.%s, *%s.%sApplyConfiguration]\n", tc.APIAlias, rs.Kind, tc.APIAlias, rs.ListKind, applyAlias, rs.Kind)
	b.WriteString("}\n\n")

	// Constructor.
	fmt.Fprintf(&b, "// new%s returns a %s\n", plural, rs.Kind)
	if rs.Namespaced {
		fmt.Fprintf(&b, "func new%s(c *%s, namespace string) *%s {\n", plural, tc.GroupClient, lowerFirst(plural))
	} else {
		fmt.Fprintf(&b, "func new%s(c *%s) *%s {\n", plural, tc.GroupClient, lowerFirst(plural))
	}
	fmt.Fprintf(&b, "\treturn &%s{\n", lowerFirst(plural))
	fmt.Fprintf(&b, "\t\tgentype.NewClientWithListAndApply[*%s.%s, *%s.%s, *%s.%sApplyConfiguration](\n", tc.APIAlias, rs.Kind, tc.APIAlias, rs.ListKind, applyAlias, rs.Kind)
	fmt.Fprintf(&b, "\t\t\t%q,\n", rs.Plural)
	b.WriteString("\t\t\tc.RESTClient(),\n")
	b.WriteString("\t\t\tscheme.ParameterCodec,\n")
	if rs.Namespaced {
		b.WriteString("\t\t\tnamespace,\n")
	} else {
		b.WriteString("\t\t\t\"\",\n")
	}
	fmt.Fprintf(&b, "\t\t\tfunc() *%s.%s { return &%s.%s{} },\n", tc.APIAlias, rs.Kind, tc.APIAlias, rs.Kind)
	fmt.Fprintf(&b, "\t\t\tfunc() *%s.%s { return &%s.%s{} },\n", tc.APIAlias, rs.ListKind, tc.APIAlias, rs.ListKind)
	b.WriteString("\t\t),\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")

	return b.String()
}

// generateInterfaceMethods writes the resource interface method declarations.
func generateInterfaceMethods(b *strings.Builder, tc *TypeContext, rs *spec.ResourceSpec) {
	t := "*" + tc.APIAlias + "." + rs.Kind
	l := "*" + tc.APIAlias + "." + rs.ListKind
	for _, verb := range rs.Verbs {
		switch verb {
		case "create":
			fmt.Fprintf(b, "\tCreate(ctx context.Context, %s %s, opts metav1.CreateOptions) (%s, error)\n", lowerFirst(rs.Kind), t, t)
		case "update":
			fmt.Fprintf(b, "\tUpdate(ctx context.Context, %s %s, opts metav1.UpdateOptions) (%s, error)\n", lowerFirst(rs.Kind), t, t)
		case "updateStatus":
			fmt.Fprintf(b, "\tUpdateStatus(ctx context.Context, %s %s, opts metav1.UpdateOptions) (%s, error)\n", lowerFirst(rs.Kind), t, t)
		case "delete":
			fmt.Fprintf(b, "\tDelete(ctx context.Context, name string, opts metav1.DeleteOptions) error\n")
		case "deleteCollection":
			fmt.Fprintf(b, "\tDeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error\n")
		case "get":
			fmt.Fprintf(b, "\tGet(ctx context.Context, name string, opts metav1.GetOptions) (%s, error)\n", t)
		case "list":
			fmt.Fprintf(b, "\tList(ctx context.Context, opts metav1.ListOptions) (%s, error)\n", l)
		case "watch":
			fmt.Fprintf(b, "\tWatch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)\n")
		case "patch":
			fmt.Fprintf(b, "\tPatch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (%s, error)\n", t)
		}
	}
	// Apply is always generated (the client embeds ClientWithListAndApply).
	applyAlias := applycfgAlias(tc)
	fmt.Fprintf(b, "\tApply(ctx context.Context, %s *%s.%sApplyConfiguration, opts metav1.ApplyOptions) (%s, error)\n", lowerFirst(rs.Kind), applyAlias, rs.Kind, t)
	if rs.HasStatus {
		fmt.Fprintf(b, "\tApplyStatus(ctx context.Context, %s *%s.%sApplyConfiguration, opts metav1.ApplyOptions) (%s, error)\n", lowerFirst(rs.Kind), applyAlias, rs.Kind, t)
	}
	fmt.Fprintf(b, "\t%sExpansion\n", rs.Kind)
}

func typeImports(tc *TypeContext, rs *spec.ResourceSpec) string {
	var b strings.Builder
	b.WriteString("import (\n")
	b.WriteString("\tcontext \"context\"\n\n")
	fmt.Fprintf(&b, "\t%s %q\n", tc.APIAlias, tc.Version.GoPackage)
	fmt.Fprintf(&b, "\t%s %q\n", applycfgAlias(tc), tc.Spec.ClientsetGoPackage+"/applyconfigurations/"+groupPkgName(tc.Group)+"/"+tc.Version.Version)
	fmt.Fprintf(&b, "\tscheme %q\n", tc.Spec.ClientsetGoPackage+"/scheme")
	b.WriteString("\tmetav1 \"k8s.io/apimachinery/pkg/apis/meta/v1\"\n")
	if hasVerb(rs, "patch") {
		b.WriteString("\ttypes \"k8s.io/apimachinery/pkg/types\"\n")
	}
	if hasVerb(rs, "watch") {
		b.WriteString("\twatch \"k8s.io/apimachinery/pkg/watch\"\n")
	}
	b.WriteString("\tgentype \"k8s.io/client-go/gentype\"\n")
	b.WriteString(")\n\n")
	return b.String()
}

// hasVerb reports whether rs.Verbs contains verb.
func hasVerb(rs *spec.ResourceSpec, verb string) bool {
	for _, v := range rs.Verbs {
		if v == verb {
			return true
		}
	}
	return false
}

// applycfgAlias returns the import alias of the apply configurations package,
// e.g. "applyconfigurationsappsv1".
func applycfgAlias(tc *TypeContext) string {
	return "applyconfigurations" + strings.ToLower(tc.Group.GroupGoName) + tc.Version.Version
}

// typedPackageName is the package name of the typed group/version directory,
// e.g. "v1".
func typedPackageName(tc *TypeContext) string {
	return tc.Version.Version
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

// pluralGoName returns the exported Go name for the resource plural, e.g.
// "Deployments" for plural "deployments".
func pluralGoName(rs *spec.ResourceSpec) string {
	return upperFirst(rs.Plural)
}
