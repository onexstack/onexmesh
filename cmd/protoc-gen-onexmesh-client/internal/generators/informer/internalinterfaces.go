// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package informer

import (
	"fmt"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// GenerateInternalInterfaces renders informers/internalinterfaces/factory_interfaces.go.
func GenerateInternalInterfaces(s *spec.Spec) string {
	var b string
	b += header
	b += "package internalinterfaces\n\n"
	b += "import (\n"
	b += "\ttime \"time\"\n\n"
	b += "\tv1 \"k8s.io/apimachinery/pkg/apis/meta/v1\"\n"
	b += "\truntime \"k8s.io/apimachinery/pkg/runtime\"\n"
	b += fmt.Sprintf("\t%s %q\n", s.ClientsetPackageName, s.ClientsetGoPackage)
	b += "\tcache \"k8s.io/client-go/tools/cache\"\n"
	b += ")\n\n"
	b += fmt.Sprintf("// NewInformerFunc takes %s.Interface and time.Duration to return a SharedIndexInformer.\n", s.ClientsetPackageName)
	b += fmt.Sprintf("type NewInformerFunc func(%s.Interface, time.Duration) cache.SharedIndexInformer\n\n", s.ClientsetPackageName)
	b += "// SharedInformerFactory a small interface to allow for adding an informer without an import cycle\n"
	b += "type SharedInformerFactory interface {\n"
	b += "\tStart(stopCh <-chan struct{})\n"
	b += "\tInformerFor(obj runtime.Object, newFunc NewInformerFunc) cache.SharedIndexInformer\n"
	b += "\tInformerName() *cache.InformerName\n"
	b += "}\n\n"
	b += "// TweakListOptionsFunc is a function that transforms a v1.ListOptions.\n"
	b += "type TweakListOptionsFunc func(*v1.ListOptions)\n\n"
	b += "// InformerOptions holds the options for creating an informer.\n"
	b += "type InformerOptions struct {\n"
	b += "\t// ResyncPeriod is the resync period for this informer.\n"
	b += "\t// If not set, defaults to 0 (no resync).\n"
	b += "\tResyncPeriod time.Duration\n\n"
	b += "\t// Indexers are the indexers for this informer.\n"
	b += "\tIndexers cache.Indexers\n\n"
	b += "\t// InformerName is used to uniquely identify this informer for metrics.\n"
	b += "\t// If not set, metrics will not be published for this informer.\n"
	b += "\t// Use cache.NewInformerName() to create an InformerName at startup.\n"
	b += "\tInformerName *cache.InformerName\n\n"
	b += "\t// TweakListOptions is an optional function to modify the list options.\n"
	b += "\tTweakListOptions TweakListOptionsFunc\n"
	b += "}\n"
	return b
}
