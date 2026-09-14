// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package applycfg generates per-version apply configuration types
// (<Kind>ApplyConfiguration) that power server-side apply through the gentype
// ClientWithListAndApply / FakeClientWithListAndApply variants.
//
// The generated types live in a dedicated applyconfigurations/<group>/<version>
// package (mirroring k8s.io/client-go/applyconfigurations), so their
// constructor functions (e.g. Deployment(name, namespace)) do not collide with
// the API type of the same name.
//
// Each generated type embeds meshmeta.TypeMetaApplyConfiguration and
// *meshmeta.ObjectMetaApplyConfiguration (so it satisfies gentype's namedObject
// constraint: comparable + GetName() *string) and declares a pointer field for
// each business field of the resource message.
//
// NOTE: this is a minimal, working subset of k8s.io/code-generator's
// applyconfiguration-gen. It does NOT generate nested <SubType>ApplyConfiguration
// types for message-typed fields, nor the Extract* functions that depend on
// structured-merge-diff. Business message fields reference the proto type
// pointer directly.
package applycfg

import (
	"fmt"
	"strings"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// Generate renders the apply configuration file for a group/version, writing
// one <Kind>ApplyConfiguration type per resource.
func Generate(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	apiAlias := apiAlias(g, vs)
	needsAPI := hasMessageField(vs)

	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package %s\n\n", vs.Version)
	b.WriteString("import (\n")
	if needsAPI {
		fmt.Fprintf(&b, "\t%s %q\n", apiAlias, vs.GoPackage)
	}
	b.WriteString("\tmeshmeta \"github.com/onexstack/onexmesh/pkg/proto/onexmesh/meta/v1\"\n")
	b.WriteString(")\n\n")

	for _, rs := range vs.Resources {
		generateResource(&b, vs, rs, apiAlias)
	}
	return b.String()
}

// hasMessageField reports whether any resource in the version has a
// message-typed business field (requiring an import of the API types package).
func hasMessageField(vs *spec.VersionSpec) bool {
	for _, rs := range vs.Resources {
		for _, f := range rs.Fields {
			if f.IsMessage {
				return true
			}
		}
	}
	return false
}

func generateResource(b *strings.Builder, vs *spec.VersionSpec, rs *spec.ResourceSpec, apiAlias string) {
	kind := rs.Kind
	cfg := kind + "ApplyConfiguration"

	fmt.Fprintf(b, "// %s represents a declarative configuration of the %s type for use\n", cfg, kind)
	b.WriteString("// with apply.\n")
	fmt.Fprintf(b, "type %s struct {\n", cfg)
	b.WriteString("\tmeshmeta.TypeMetaApplyConfiguration `json:\",inline\"`\n")
	b.WriteString("\t*meshmeta.ObjectMetaApplyConfiguration `json:\"metadata,omitempty\"`\n")
	for _, f := range rs.Fields {
		fmt.Fprintf(b, "\t%s %s `json:\"%s,omitempty\"`\n", f.GoName, fieldType(f, apiAlias), jsonName(f))
	}
	b.WriteString("}\n\n")

	// Constructor.
	fmt.Fprintf(b, "// %s constructs a declarative configuration of the %s type for use with\n", kind, kind)
	b.WriteString("// apply.\n")
	if rs.Namespaced {
		fmt.Fprintf(b, "func %s(name, namespace string) *%s {\n", kind, cfg)
	} else {
		fmt.Fprintf(b, "func %s(name string) *%s {\n", kind, cfg)
	}
	fmt.Fprintf(b, "\tb := &%s{}\n", cfg)
	b.WriteString("\tb.WithName(name)\n")
	if rs.Namespaced {
		b.WriteString("\tb.WithNamespace(namespace)\n")
	}
	fmt.Fprintf(b, "\tb.WithKind(%q)\n", kind)
	fmt.Fprintf(b, "\tb.WithAPIVersion(%q)\n", apiVersion(vs))
	b.WriteString("\treturn b\n")
	b.WriteString("}\n\n")

	// With<Field> methods for business fields.
	for _, f := range rs.Fields {
		fmt.Fprintf(b, "// With%s sets the %s field in the declarative configuration to the given\n", f.GoName, f.GoName)
		b.WriteString("// value and returns the receiver, so that objects can be built by chaining\n")
		b.WriteString("// \"With\" function invocations.\n")
		fmt.Fprintf(b, "func (b *%s) With%s(value %s) *%s {\n", cfg, f.GoName, valueType(f, apiAlias), cfg)
		fmt.Fprintf(b, "\tb.%s = %s\n", f.GoName, assignExpr(f))
		b.WriteString("\treturn b\n")
		b.WriteString("}\n\n")
	}

	// Metadata override methods: the embedded *ObjectMetaApplyConfiguration is
	// nil until first use, so each of these ensures it exists before forwarding.
	generateMetaOverrides(b, cfg)
}

// generateMetaOverrides emits the WithName/WithNamespace/... overrides that
// lazily initialize the embedded *meshmeta.ObjectMetaApplyConfiguration.
func generateMetaOverrides(b *strings.Builder, cfg string) {
	ensure := func() {
		fmt.Fprintf(b, "func (b *%s) ensureObjectMetaApplyConfigurationExists() {\n", cfg)
		b.WriteString("\tif b.ObjectMetaApplyConfiguration == nil {\n")
		b.WriteString("\t\tb.ObjectMetaApplyConfiguration = &meshmeta.ObjectMetaApplyConfiguration{}\n")
		b.WriteString("\t}\n")
		b.WriteString("}\n\n")
	}
	ensure()

	// Scalar metadata overrides.
	for _, name := range []string{"Name", "GenerateName", "Namespace"} {
		fmt.Fprintf(b, "// With%s sets the %s field in the declarative configuration to the given\n", name, name)
		b.WriteString("// value and returns the receiver, so that objects can be built by chaining\n")
		b.WriteString("// \"With\" function invocations.\n")
		fmt.Fprintf(b, "func (b *%s) With%s(value string) *%s {\n", cfg, name, cfg)
		b.WriteString("\tb.ensureObjectMetaApplyConfigurationExists()\n")
		fmt.Fprintf(b, "\tb.ObjectMetaApplyConfiguration.With%s(value)\n", name)
		b.WriteString("\treturn b\n")
		b.WriteString("}\n\n")
	}

	// Map metadata overrides.
	for _, name := range []string{"Labels", "Annotations"} {
		fmt.Fprintf(b, "// With%s puts the entries into the %s field in the declarative configuration\n", name, name)
		b.WriteString("// and returns the receiver.\n")
		fmt.Fprintf(b, "func (b *%s) With%s(entries map[string]string) *%s {\n", cfg, name, cfg)
		b.WriteString("\tb.ensureObjectMetaApplyConfigurationExists()\n")
		fmt.Fprintf(b, "\tb.ObjectMetaApplyConfiguration.With%s(entries)\n", name)
		b.WriteString("\treturn b\n")
		b.WriteString("}\n\n")
	}

	// Finalizers override.
	b.WriteString("// WithFinalizers adds the given value to the Finalizers field in the declarative\n")
	b.WriteString("// configuration and returns the receiver.\n")
	fmt.Fprintf(b, "func (b *%s) WithFinalizers(values ...string) *%s {\n", cfg, cfg)
	b.WriteString("\tb.ensureObjectMetaApplyConfigurationExists()\n")
	b.WriteString("\tb.ObjectMetaApplyConfiguration.WithFinalizers(values...)\n")
	b.WriteString("\treturn b\n")
	b.WriteString("}\n\n")
}

// fieldType returns the field declaration type, package-qualifying message
// types and pointer-qualifying scalars.
func fieldType(f spec.FieldSpec, apiAlias string) string {
	if f.IsMessage {
		return "*" + apiAlias + "." + strings.TrimPrefix(f.GoType, "*")
	}
	if strings.HasPrefix(f.GoType, "[]") || strings.HasPrefix(f.GoType, "map[") {
		return f.GoType
	}
	return "*" + f.GoType
}

// valueType returns the parameter type of With<Field>: scalars pass by value,
// message/slice/map types pass by reference.
func valueType(f spec.FieldSpec, apiAlias string) string {
	if f.IsMessage {
		return "*" + apiAlias + "." + strings.TrimPrefix(f.GoType, "*")
	}
	return f.GoType
}

// assignExpr returns the right-hand side of the With<Field> assignment:
// scalars need a pointer to the (value) parameter; reference types assign the
// parameter directly.
func assignExpr(f spec.FieldSpec) string {
	if f.IsMessage || strings.HasPrefix(f.GoType, "[]") || strings.HasPrefix(f.GoType, "map[") {
		return "value"
	}
	return "&value"
}

// jsonName returns the JSON tag name (the proto field name, matching the
// resource message's json tags since the resource uses camelCase field names).
func jsonName(f spec.FieldSpec) string {
	return f.ProtoName
}

func apiVersion(vs *spec.VersionSpec) string {
	if vs.Group == "" {
		return vs.Version
	}
	return vs.Group + "/" + vs.Version
}

// apiAlias returns the import alias of the API types package, e.g. "appsv1".
func apiAlias(g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return strings.ToLower(g.GroupGoName) + vs.Version
}

const header = `// Code generated by protoc-gen-onexmesh-client. DO NOT EDIT.

`
