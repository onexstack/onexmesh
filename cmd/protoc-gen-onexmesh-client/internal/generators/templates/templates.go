// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package templates renders the client-go style SDK source from the spec IR via
// text/template. Each generated file has a corresponding *.tmpl file embedded
// here; generators call Render(name, view) with the spec IR (or a small view
// struct) and the shared FuncMap below supplies every naming/type helper,
// eliminating the per-package duplication the string-concatenation generators
// used to carry.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

//go:embed *.tmpl
var files embed.FS

// funcs is the shared template FuncMap. Every helper is a pure function over
// spec IR values, so a single map replaces the many unexported copies that used
// to live in typed, fake, informer, lister, applycfg and scheme.
var funcs = template.FuncMap{
	"slice":              func(v ...any) []any { return v },
	"upperFirst":         upperFirst,
	"lowerFirst":         lowerFirst,
	"pluralGoName":       func(rs *spec.ResourceSpec) string { return upperFirst(rs.Plural) },
	"groupVersionGoName": groupVersionGoName,
	"versionGoName":      func(vs *spec.VersionSpec) string { return upperFirst(vs.Version) },
	"resourceAccessorName": func(rs *spec.ResourceSpec, vs *spec.VersionSpec) string {
		return rs.Kind + upperFirst(vs.Version)
	},
	"groupPkgName":        groupPkgName,
	"apiAlias":            apiAlias,
	"apiAliasWithPrefix":  func(g *spec.GroupSpec, vs *spec.VersionSpec) string { return "api" + apiAlias(g, vs) },
	"applycfgAlias":       applycfgAlias,
	"typedPkgAlias":       func(g *spec.GroupSpec, vs *spec.VersionSpec) string { return "typed" + apiAlias(g, vs) },
	"typedPkgPath":        typedPkgPath,
	"applycfgPath":        applycfgPath,
	"listerPath":          listerPath,
	"informersRoot":       func(s *spec.Spec) string { return s.ClientsetGoPackage + "/informers" },
	"groupInformerPath":   groupInformerPath,
	"versionInformerPath": versionInformerPath,
	"fakeGroupName": func(g *spec.GroupSpec, vs *spec.VersionSpec) string {
		return "Fake" + g.GroupGoName + upperFirst(vs.Version)
	},
	"hasVerb":              hasVerb,
	"hasMessageField":      hasMessageField,
	"listMetaNeeded":       listMetaNeeded,
	"apiPath":              apiPath,
	"apiVersion":           apiVersion,
	"resourceAccessorExpr": resourceAccessorExpr,
	"nsParam":              nsParam,
	"nsArg":                nsArg,
	"nsFieldArg":           nsFieldArg,
	"fieldType":            fieldType,
	"valueType":            valueType,
	"assignExpr":           assignExpr,
	"jsonName":             func(f spec.FieldSpec) string { return f.ProtoName },
	"typedPackageName":     func(vs *spec.VersionSpec) string { return vs.Version },
}

// all holds the parsed templates, indexed by file name (e.g. "mesh.tmpl").
var all = func() *template.Template {
	return template.Must(template.New("").Funcs(funcs).ParseFS(files, "*.tmpl"))
}()

// Render executes the named template (without the .tmpl suffix) against view
// and returns the rendered source. Parse errors are caught at package init;
// execution errors are programming errors and panic.
func Render(name string, view any) string {
	var b bytes.Buffer
	if err := all.ExecuteTemplate(&b, name+".tmpl", view); err != nil {
		panic(fmt.Sprintf("render template %s: %v", name, err))
	}
	return b.String()
}

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

func groupVersionGoName(g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return g.GroupGoName + upperFirst(vs.Version)
}

func groupPkgName(g *spec.GroupSpec) string {
	if g.Group == "" {
		return "core"
	}
	return g.Group
}

func apiAlias(g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return strings.ToLower(g.GroupGoName) + vs.Version
}

func applycfgAlias(g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return "applyconfigurations" + strings.ToLower(g.GroupGoName) + vs.Version
}

func typedPkgPath(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return s.ClientsetGoPackage + "/typed/" + groupPkgName(g) + "/" + vs.Version
}

func applycfgPath(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return s.ClientsetGoPackage + "/applyconfigurations/" + groupPkgName(g) + "/" + vs.Version
}

func listerPath(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return s.ClientsetGoPackage + "/listers/" + groupPkgName(g) + "/" + vs.Version
}

func groupInformerPath(s *spec.Spec, g *spec.GroupSpec) string {
	return s.ClientsetGoPackage + "/informers/" + groupPkgName(g)
}

func versionInformerPath(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return groupInformerPath(s, g) + "/" + vs.Version
}

func hasVerb(rs *spec.ResourceSpec, verb string) bool {
	for _, v := range rs.Verbs {
		if v == verb {
			return true
		}
	}
	return false
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

// listMetaNeeded reports whether any resource in the version has a list kind,
// which requires the shared meshListMetaAdapter.
func listMetaNeeded(vs *spec.VersionSpec) bool {
	for _, rs := range vs.Resources {
		if rs.ListKind != "" {
			return true
		}
	}
	return false
}

// apiPath returns "/api" for the core (empty) group and the clientset API path
// (default "/apis") for every other group, mirroring client-gen.
func apiPath(s *spec.Spec, g *spec.GroupSpec) string {
	if g.Group == "" {
		return "/api"
	}
	return s.ClientsetAPIPath
}

func apiVersion(vs *spec.VersionSpec) string {
	if vs.Group == "" {
		return vs.Version
	}
	return vs.Group + "/" + vs.Version
}

// resourceAccessorExpr returns the accessor expression (without the leading
// "client.") used inside informer callbacks to reach a resource interface.
func resourceAccessorExpr(g *spec.GroupSpec, vs *spec.VersionSpec, rs *spec.ResourceSpec) string {
	if g.Group == "" {
		acc := rs.Kind + upperFirst(vs.Version)
		if rs.Namespaced {
			return acc + "(namespace)"
		}
		return acc + "()"
	}
	acc := g.GroupGoName + upperFirst(vs.Version)
	plural := upperFirst(rs.Plural)
	if rs.Namespaced {
		return acc + "()." + plural + "(namespace)"
	}
	return acc + "()." + plural + "()"
}

func nsParam(rs *spec.ResourceSpec) string {
	if rs.Namespaced {
		return ", namespace string"
	}
	return ""
}

func nsArg(rs *spec.ResourceSpec) string {
	if rs.Namespaced {
		return ", namespace"
	}
	return ""
}

func nsFieldArg(rs *spec.ResourceSpec) string {
	if rs.Namespaced {
		return ", f.namespace"
	}
	return ""
}

func fieldType(f spec.FieldSpec, apiAlias string) string {
	if f.IsMessage {
		return "*" + apiAlias + "." + strings.TrimPrefix(f.GoType, "*")
	}
	if strings.HasPrefix(f.GoType, "[]") || strings.HasPrefix(f.GoType, "map[") {
		return f.GoType
	}
	return "*" + f.GoType
}

func valueType(f spec.FieldSpec, apiAlias string) string {
	if f.IsMessage {
		return "*" + apiAlias + "." + strings.TrimPrefix(f.GoType, "*")
	}
	return f.GoType
}

func assignExpr(f spec.FieldSpec) string {
	if f.IsMessage || strings.HasPrefix(f.GoType, "[]") || strings.HasPrefix(f.GoType, "map[") {
		return "value"
	}
	return "&value"
}
