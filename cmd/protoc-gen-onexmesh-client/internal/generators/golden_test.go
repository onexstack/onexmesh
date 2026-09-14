// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package generators holds golden-file regression tests for the generators.
// Each generator is a pure function (func(...) string), so its output is
// deterministic; these tests lock the gofmt-ed output (the exact bytes that
// land on disk) to catch unintended template drift.
package generators

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/applycfg"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/fake"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/informer"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/lister"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/meta"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/register"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/scheme"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/generators/typed"
	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

var update = flag.Bool("update", false, "update golden files")

// exampleSpec builds the shared fixture: an apps/v1 group (Deployment +
// DaemonSet, namespaced) and a core/v1 group (Pod namespaced, Node
// cluster-scoped), aggregated into the "exampleclient" clientset.
func exampleSpec() *spec.Spec {
	deployment := &spec.ResourceSpec{
		Kind: "Deployment", Plural: "deployments", Singular: "deployment",
		Namespaced:      true,
		Verbs:           []string{"get", "list", "watch", "create", "update", "patch", "delete", "updateStatus"},
		HasStatus:       true,
		ListKind:        "DeploymentList",
		APIVersionField: "ApiVersion", KindField: "Kind", MetadataField: "Metadata",
	}
	daemonset := &spec.ResourceSpec{
		Kind: "DaemonSet", Plural: "daemonsets", Singular: "daemonset",
		Namespaced:      true,
		Verbs:           []string{"get", "list", "watch", "create", "update", "patch", "delete", "updateStatus"},
		HasStatus:       true,
		ListKind:        "DaemonSetList",
		APIVersionField: "ApiVersion", KindField: "Kind", MetadataField: "Metadata",
	}
	pod := &spec.ResourceSpec{
		Kind: "Pod", Plural: "pods", Singular: "pod",
		Namespaced:      true,
		Verbs:           []string{"get", "list", "watch", "create", "update", "patch", "delete", "updateStatus"},
		HasStatus:       true,
		ListKind:        "PodList",
		APIVersionField: "ApiVersion", KindField: "Kind", MetadataField: "Metadata",
	}
	node := &spec.ResourceSpec{
		Kind: "Node", Plural: "nodes", Singular: "node",
		Namespaced:      false, // cluster-scoped
		Verbs:           []string{"get", "list", "watch", "create", "update", "patch", "delete", "updateStatus"},
		HasStatus:       true,
		ListKind:        "NodeList",
		APIVersionField: "ApiVersion", KindField: "Kind", MetadataField: "Metadata",
	}

	return &spec.Spec{
		ClientsetName:        "exampleclient",
		ClientsetGoName:      "Exampleclient",
		ClientsetAPIPath:     "/apis",
		ClientsetGoPackage:   "github.com/onexstack/onexmesh/examples/pkg/generated/exampleclient",
		ClientsetPackageName: "exampleclient",
		ClientsetProtoDir:    "examples/pkg/generated/exampleclient",
		Groups: []*spec.GroupSpec{
			{
				Group: "apps", GroupGoName: "Apps",
				Versions: []*spec.VersionSpec{
					{
						Group: "apps", Version: "v1",
						GoPackage:     "github.com/onexstack/onexmesh/examples/apis/apps/v1",
						GoPackageName: "v1",
						ProtoDir:      "examples/apis/apps/v1",
						Resources:     []*spec.ResourceSpec{daemonset, deployment},
					},
				},
			},
			{
				Group: "", GroupGoName: "Core",
				Versions: []*spec.VersionSpec{
					{
						Group: "", Version: "v1",
						GoPackage:     "github.com/onexstack/onexmesh/examples/apis/core/v1",
						GoPackageName: "v1",
						ProtoDir:      "examples/apis/core/v1",
						Resources:     []*spec.ResourceSpec{node, pod},
					},
				},
			},
		},
	}
}

// golden compares got (after gofmt, matching what lands on disk) against
// testdata/<name>; with -update it rewrites the file.
func golden(t *testing.T, name, got string) {
	t.Helper()
	formatted, err := FormatSource(name, got)
	if err != nil {
		t.Fatalf("format generated source %s: %v", name, err)
	}
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(formatted), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if formatted != string(want) {
		t.Errorf("golden %s mismatch (run with -update to regenerate):\n--- got ---\n%s\n--- want ---\n%s", path, formatted, want)
	}
}

func TestGoldenClientset(t *testing.T) {
	golden(t, "clientset.golden", typed.GenerateClientset(exampleSpec()))
}

func TestGoldenScheme(t *testing.T) {
	golden(t, "scheme.golden", scheme.Generate(exampleSpec()))
}

func TestGoldenTypedGroup(t *testing.T) {
	s := exampleSpec()
	tc := typed.NewTypeContext(s, s.Groups[0], s.Groups[0].Versions[0])
	golden(t, "typed_group.golden", typed.GenerateGroup(tc))
}

func TestGoldenTypedType(t *testing.T) {
	s := exampleSpec()
	g := s.Groups[0]
	vs := g.Versions[0]
	tc := typed.NewTypeContext(s, g, vs)
	golden(t, "typed_type.golden", typed.GenerateType(tc, vs.Resources[1])) // Deployment
}

func TestGoldenFakeClientset(t *testing.T) {
	golden(t, "fake_clientset.golden", fake.GenerateClientset(exampleSpec()))
}

func TestGoldenMeta(t *testing.T) {
	s := exampleSpec()
	golden(t, "meta.golden", meta.Generate(s.Groups[0].Versions[0]))
}

func TestGoldenRegister(t *testing.T) {
	s := exampleSpec()
	golden(t, "register.golden", register.Generate(s.Groups[0].Versions[0]))
}

func TestGoldenLister(t *testing.T) {
	s := exampleSpec()
	g := s.Groups[0]
	vs := g.Versions[0]
	golden(t, "lister.golden", lister.GenerateLister(s, g, vs, vs.Resources[1])) // Deployment
}

func TestGoldenInformerFactory(t *testing.T) {
	golden(t, "informer_factory.golden", informer.GenerateFactory(exampleSpec()))
}

func TestGoldenInformer(t *testing.T) {
	s := exampleSpec()
	g := s.Groups[0]
	vs := g.Versions[0]
	golden(t, "informer.golden", informer.GenerateInformer(s, g, vs, vs.Resources[1])) // Deployment
}

func TestGoldenGeneric(t *testing.T) {
	golden(t, "generic.golden", informer.GenerateGeneric(exampleSpec()))
}

func TestGoldenApplyConfiguration(t *testing.T) {
	s := exampleSpec()
	g := s.Groups[0]
	vs := g.Versions[0]
	golden(t, "applyconfiguration.golden", applycfg.Generate(s, g, vs))
}
