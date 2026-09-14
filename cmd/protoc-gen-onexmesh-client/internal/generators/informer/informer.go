// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package informer

import (
	"fmt"
	"strings"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// GenerateInformer renders a single resource informer file.
func GenerateInformer(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec, rs *spec.ResourceSpec) string {
	api := apiAliasWithPrefix(g, vs)
	listerAlias := apiAlias(g, vs)
	clientset := s.ClientsetPackageName
	kind := rs.Kind
	pluralGo := upperFirst(rs.Plural)
	lowerKind := lowerFirst(kind)
	groupVersionAccessor := g.GroupGoName + versionGoName(vs) // e.g. "AppsV1"

	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package %s\n\n", vs.Version)
	b.WriteString(informerImports(s, g, vs))

	// Interface.
	fmt.Fprintf(&b, "// %sInformer provides access to a shared informer and lister for\n", kind)
	fmt.Fprintf(&b, "// %s.\n", pluralGo)
	fmt.Fprintf(&b, "type %sInformer interface {\n", kind)
	b.WriteString("\tInformer() cache.SharedIndexInformer\n")
	fmt.Fprintf(&b, "\tLister() %s.%sLister\n", listerAlias, kind)
	b.WriteString("}\n\n")

	// Struct.
	fmt.Fprintf(&b, "type %sInformer struct {\n", lowerKind)
	b.WriteString("\tfactory          internalinterfaces.SharedInformerFactory\n")
	b.WriteString("\ttweakListOptions internalinterfaces.TweakListOptionsFunc\n")
	if rs.Namespaced {
		b.WriteString("\tnamespace        string\n")
	}
	b.WriteString("}\n\n")

	// Public constructor.
	fmt.Fprintf(&b, "// New%sInformer constructs a new informer for %s type.\n", kind, kind)
	b.WriteString("// Always prefer using an informer factory to get a shared informer instead of getting an independent\n")
	b.WriteString("// one. This reduces memory footprint and number of connections to the server.\n")
	fmt.Fprintf(&b, "func New%sInformer(client %s.Interface%s, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {\n", kind, clientset, nsParam(rs))
	fmt.Fprintf(&b, "\treturn New%sInformerWithOptions(client%s, internalinterfaces.InformerOptions{ResyncPeriod: resyncPeriod, Indexers: indexers})\n", kind, nsArg(rs))
	b.WriteString("}\n\n")

	// Filtered public constructor.
	fmt.Fprintf(&b, "// NewFiltered%sInformer constructs a new informer for %s type.\n", kind, kind)
	b.WriteString("// Always prefer using an informer factory to get a shared informer instead of getting an independent\n")
	b.WriteString("// one. This reduces memory footprint and number of connections to the server.\n")
	fmt.Fprintf(&b, "func NewFiltered%sInformer(client %s.Interface%s, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {\n", kind, clientset, nsParam(rs))
	fmt.Fprintf(&b, "\treturn New%sInformerWithOptions(client%s, internalinterfaces.InformerOptions{ResyncPeriod: resyncPeriod, Indexers: indexers, TweakListOptions: tweakListOptions})\n", kind, nsArg(rs))
	b.WriteString("}\n\n")

	// WithOptions constructor.
	fmt.Fprintf(&b, "// New%sInformerWithOptions constructs a new informer for %s type with additional options.\n", kind, kind)
	b.WriteString("// Always prefer using an informer factory to get a shared informer instead of getting an independent\n")
	b.WriteString("// one. This reduces memory footprint and number of connections to the server.\n")
	fmt.Fprintf(&b, "func New%sInformerWithOptions(client %s.Interface%s, options internalinterfaces.InformerOptions) cache.SharedIndexInformer {\n", kind, clientset, nsParam(rs))
	fmt.Fprintf(&b, "\tgvr := schema.GroupVersionResource{Group: %q, Version: %q, Resource: %q}\n", g.Group, vs.Version, rs.Plural)
	b.WriteString("\tidentifier := options.InformerName.WithResource(gvr)\n")
	b.WriteString("\ttweakListOptions := options.TweakListOptions\n")
	b.WriteString("\treturn cache.NewSharedIndexInformerWithOptions(\n")
	b.WriteString("\t\tcache.ToListWatcherWithWatchListSemantics(&cache.ListWatch{\n")
	b.WriteString("\t\t\tListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {\n")
	b.WriteString("\t\t\t\tif tweakListOptions != nil {\n")
	b.WriteString("\t\t\t\t\ttweakListOptions(&opts)\n")
	b.WriteString("\t\t\t\t}\n")
	fmt.Fprintf(&b, "\t\t\t\treturn client.%s().%s(%s).List(context.Background(), opts)\n", groupVersionAccessor, pluralGo, nsGetterArg(rs))
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t\tWatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {\n")
	b.WriteString("\t\t\t\tif tweakListOptions != nil {\n")
	b.WriteString("\t\t\t\t\ttweakListOptions(&opts)\n")
	b.WriteString("\t\t\t\t}\n")
	fmt.Fprintf(&b, "\t\t\t\treturn client.%s().%s(%s).Watch(context.Background(), opts)\n", groupVersionAccessor, pluralGo, nsGetterArg(rs))
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t\tListWithContextFunc: func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {\n")
	b.WriteString("\t\t\t\tif tweakListOptions != nil {\n")
	b.WriteString("\t\t\t\t\ttweakListOptions(&opts)\n")
	b.WriteString("\t\t\t\t}\n")
	fmt.Fprintf(&b, "\t\t\t\treturn client.%s().%s(%s).List(ctx, opts)\n", groupVersionAccessor, pluralGo, nsGetterArg(rs))
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t\tWatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {\n")
	b.WriteString("\t\t\t\tif tweakListOptions != nil {\n")
	b.WriteString("\t\t\t\t\ttweakListOptions(&opts)\n")
	b.WriteString("\t\t\t\t}\n")
	fmt.Fprintf(&b, "\t\t\t\treturn client.%s().%s(%s).Watch(ctx, opts)\n", groupVersionAccessor, pluralGo, nsGetterArg(rs))
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t}, client),\n")
	fmt.Fprintf(&b, "\t\t&%s.%s{},\n", api, kind)
	b.WriteString("\t\tcache.SharedIndexInformerOptions{\n")
	b.WriteString("\t\t\tResyncPeriod: options.ResyncPeriod,\n")
	b.WriteString("\t\t\tIndexers:     options.Indexers,\n")
	b.WriteString("\t\t\tIdentifier:   identifier,\n")
	b.WriteString("\t\t},\n")
	b.WriteString("\t)\n")
	b.WriteString("}\n\n")

	// defaultInformer.
	fmt.Fprintf(&b, "func (f *%sInformer) defaultInformer(client %s.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {\n", lowerKind, clientset)
	fmt.Fprintf(&b, "\treturn New%sInformerWithOptions(client%s, internalinterfaces.InformerOptions{ResyncPeriod: resyncPeriod, Indexers: cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, InformerName: f.factory.InformerName(), TweakListOptions: f.tweakListOptions})\n", kind, nsFieldArg(rs))
	b.WriteString("}\n\n")

	// Informer.
	fmt.Fprintf(&b, "func (f *%sInformer) Informer() cache.SharedIndexInformer {\n", lowerKind)
	fmt.Fprintf(&b, "\treturn f.factory.InformerFor(&%s.%s{}, f.defaultInformer)\n", api, kind)
	b.WriteString("}\n\n")

	// Lister.
	fmt.Fprintf(&b, "func (f *%sInformer) Lister() %s.%sLister {\n", lowerKind, listerAlias, kind)
	fmt.Fprintf(&b, "\treturn %s.New%sLister(f.Informer().GetIndexer())\n", listerAlias, kind)
	b.WriteString("}\n")

	return b.String()
}

// nsParam returns the namespace parameter for a constructor signature, e.g.
// ", namespace string".
func nsParam(rs *spec.ResourceSpec) string {
	if rs.Namespaced {
		return ", namespace string"
	}
	return ""
}

// nsArg returns the namespace argument for a call site with leading comma,
// e.g. ", namespace".
func nsArg(rs *spec.ResourceSpec) string {
	if rs.Namespaced {
		return ", namespace"
	}
	return ""
}

// nsFieldArg returns the namespace argument for a call site referencing the
// informer struct field f.namespace, with leading comma, e.g. ", f.namespace".
func nsFieldArg(rs *spec.ResourceSpec) string {
	if rs.Namespaced {
		return ", f.namespace"
	}
	return ""
}

// nsGetterArg returns the namespace argument for a resource getter call
// without a leading comma, e.g. "namespace".
func nsGetterArg(rs *spec.ResourceSpec) string {
	if rs.Namespaced {
		return "namespace"
	}
	return ""
}

func informerImports(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	var b strings.Builder
	b.WriteString("import (\n")
	b.WriteString("\tcontext \"context\"\n")
	b.WriteString("\ttime \"time\"\n\n")
	fmt.Fprintf(&b, "\t%s %q\n", apiAliasWithPrefix(g, vs), vs.GoPackage)
	b.WriteString("\tmetav1 \"k8s.io/apimachinery/pkg/apis/meta/v1\"\n")
	b.WriteString("\truntime \"k8s.io/apimachinery/pkg/runtime\"\n")
	b.WriteString("\tschema \"k8s.io/apimachinery/pkg/runtime/schema\"\n")
	b.WriteString("\twatch \"k8s.io/apimachinery/pkg/watch\"\n")
	fmt.Fprintf(&b, "\tinternalinterfaces %q\n", informersRoot(s)+"/internalinterfaces")
	fmt.Fprintf(&b, "\t%s %q\n", s.ClientsetPackageName, s.ClientsetGoPackage)
	fmt.Fprintf(&b, "\t%s %q\n", apiAlias(g, vs), listerPath(s, g, vs))
	b.WriteString("\tcache \"k8s.io/client-go/tools/cache\"\n")
	b.WriteString(")\n\n")
	return b.String()
}
