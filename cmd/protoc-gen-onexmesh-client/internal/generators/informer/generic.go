// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package informer

import (
	"fmt"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// GenerateGeneric renders informers/generic.go (the ForResource switch).
func GenerateGeneric(s *spec.Spec) string {
	var b string
	b += header
	b += "package informers\n\n"
	b += "import (\n"
	b += "\tfmt \"fmt\"\n\n"
	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			b += fmt.Sprintf("\t%s %q\n", apiAlias(g, vs), vs.GoPackage)
		}
	}
	b += "\tschema \"k8s.io/apimachinery/pkg/runtime/schema\"\n"
	b += "\tcache \"k8s.io/client-go/tools/cache\"\n"
	b += ")\n\n"

	b += "// GenericInformer is type of SharedIndexInformer which will locate and delegate to other\n"
	b += "// sharedInformers based on type\n"
	b += "type GenericInformer interface {\n"
	b += "\tInformer() cache.SharedIndexInformer\n"
	b += "\tLister() cache.GenericLister\n"
	b += "}\n\n"

	b += "type genericInformer struct {\n"
	b += "\tinformer cache.SharedIndexInformer\n"
	b += "\tresource schema.GroupResource\n"
	b += "}\n\n"

	b += "// Informer returns the SharedIndexInformer.\n"
	b += "func (f *genericInformer) Informer() cache.SharedIndexInformer {\n"
	b += "\treturn f.informer\n"
	b += "}\n\n"

	b += "// Lister returns the GenericLister.\n"
	b += "func (f *genericInformer) Lister() cache.GenericLister {\n"
	b += "\treturn cache.NewGenericLister(f.Informer().GetIndexer(), f.resource)\n"
	b += "}\n\n"

	b += "// ForResource gives generic access to a shared informer of the matching type\n"
	b += "// TODO extend this to unknown resources with a client pool\n"
	b += "func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {\n"
	b += "\tswitch resource {\n"
	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			b += fmt.Sprintf("\t// Group=%s, Version=%s\n", g.Group, vs.Version)
			for _, rs := range vs.Resources {
				b += fmt.Sprintf("\tcase %s.SchemeGroupVersion.WithResource(%q):\n", apiAlias(g, vs), rs.Plural)
				b += fmt.Sprintf("\t\treturn &genericInformer{resource: resource.GroupResource(), informer: f.%s().%s().%s().Informer()}, nil\n", g.GroupGoName, versionGoName(vs), upperFirst(rs.Plural))
			}
			b += "\n"
		}
	}
	b += "\t}\n\n"
	b += "\treturn nil, fmt.Errorf(\"no informer found for %v\", resource)\n"
	b += "}\n"

	return b
}
