// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package typed

import (
	"fmt"
	"strings"

	"github.com/onexstack/onexmesh/cmd/protoc-gen-onexmesh-client/internal/spec"
)

// GenerateClientset renders the top-level clientset.go.
func GenerateClientset(s *spec.Spec) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package %s\n\n", s.ClientsetPackageName)
	b.WriteString(clientsetImports(s))

	// Interface.
	fmt.Fprintf(&b, "// Interface is the API interface for this clientset.\n")
	b.WriteString("type Interface interface {\n")
	b.WriteString("\tDiscovery() discovery.DiscoveryInterface\n")
	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			typedAlias := typedAlias(g, vs)
			gvName := groupVersionGoName(g, vs)
			fmt.Fprintf(&b, "\t%s() %s.%s\n", gvName, typedAlias, gvName+"Interface")
		}
	}
	b.WriteString("}\n\n")

	// Clientset struct.
	fmt.Fprintf(&b, "// Clientset contains the clients for groups.\n")
	b.WriteString("type Clientset struct {\n")
	b.WriteString("\t*discovery.DiscoveryClient\n")
	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			typedAlias := typedAlias(g, vs)
			gvName := groupVersionGoName(g, vs)
			fmt.Fprintf(&b, "\t%s *%s.%s\n", lowerFirst(gvName), typedAlias, gvName+"Client")
		}
	}
	b.WriteString("}\n\n")

	// Discovery + group accessors.
	b.WriteString("func (c *Clientset) Discovery() discovery.DiscoveryInterface {\n")
	b.WriteString("\tif c == nil { return nil }\n")
	b.WriteString("\treturn c.DiscoveryClient\n")
	b.WriteString("}\n\n")
	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			typedAlias := typedAlias(g, vs)
			gvName := groupVersionGoName(g, vs)
			iface := gvName + "Interface"
			fmt.Fprintf(&b, "func (c *Clientset) %s() %s.%s {\n", gvName, typedAlias, iface)
			b.WriteString("\tif c == nil { return nil }\n")
			fmt.Fprintf(&b, "\treturn c.%s\n", lowerFirst(gvName))
			b.WriteString("}\n\n")
		}
	}

	generateClientsetConstructors(&b, s)

	return b.String()
}

func generateClientsetConstructors(b *strings.Builder, s *spec.Spec) {
	// NewForConfig.
	b.WriteString("func NewForConfig(c *rest.Config) (*Clientset, error) {\n")
	b.WriteString("\tconfig := *c\n")
	b.WriteString("\tif config.UserAgent == \"\" {\n")
	b.WriteString("\t\tconfig.UserAgent = rest.DefaultKubernetesUserAgent()\n")
	b.WriteString("\t}\n")
	b.WriteString("\thttpClient, err := rest.HTTPClientFor(&config)\n")
	b.WriteString("\tif err != nil { return nil, err }\n")
	b.WriteString("\treturn NewForConfigAndClient(&config, httpClient)\n")
	b.WriteString("}\n\n")

	// NewForConfigAndClient.
	b.WriteString("func NewForConfigAndClient(c *rest.Config, h *http.Client) (*Clientset, error) {\n")
	b.WriteString("\tconfig := *c\n")
	b.WriteString("\tif config.RateLimiter == nil && config.QPS > 0 {\n")
	b.WriteString("\t\tif config.Burst <= 0 {\n")
	b.WriteString("\t\t\treturn nil, fmt.Errorf(\"burst is required to be greater than 0 when RateLimiter is not set and QPS is set to greater than 0\")\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\tconfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(config.QPS, config.Burst)\n")
	b.WriteString("\t}\n")
	b.WriteString("\tvar cs Clientset\n")
	b.WriteString("\tvar err error\n")
	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			typedAlias := typedAlias(g, vs)
			gvName := groupVersionGoName(g, vs)
			fmt.Fprintf(b, "\tcs.%s, err = %s.NewForConfigAndClient(&config, h)\n", lowerFirst(gvName), typedAlias)
			b.WriteString("\tif err != nil { return nil, err }\n")
		}
	}
	b.WriteString("\tcs.DiscoveryClient, err = discovery.NewDiscoveryClientForConfigAndClient(&config, h)\n")
	b.WriteString("\tif err != nil { return nil, err }\n")
	b.WriteString("\treturn &cs, nil\n")
	b.WriteString("}\n\n")

	// NewForConfigOrDie.
	b.WriteString("func NewForConfigOrDie(c *rest.Config) *Clientset {\n")
	b.WriteString("\tcs, err := NewForConfig(c)\n")
	b.WriteString("\tif err != nil { panic(err) }\n")
	b.WriteString("\treturn cs\n")
	b.WriteString("}\n\n")

	// New.
	b.WriteString("func New(c rest.Interface) *Clientset {\n")
	b.WriteString("\tvar cs Clientset\n")
	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			typedAlias := typedAlias(g, vs)
			gvName := groupVersionGoName(g, vs)
			fmt.Fprintf(b, "\tcs.%s = %s.New(c)\n", lowerFirst(gvName), typedAlias)
		}
	}
	b.WriteString("\tcs.DiscoveryClient = discovery.NewDiscoveryClient(c)\n")
	b.WriteString("\treturn &cs\n")
	b.WriteString("}\n\n")

	// NewForMesh discovers the logical service via the onexmesh registry and
	// returns a Clientset whose requests are load-balanced across its instances.
	b.WriteString("// NewForMesh discovers the service via the onexmesh registry and returns a\n")
	b.WriteString("// Clientset whose requests are load-balanced across its instances.\n")
	b.WriteString("func NewForMesh(serviceName string, opts ...meshrest.Option) (*Clientset, error) {\n")
	b.WriteString("\tcfg, err := meshrest.NewForMeshConfig(serviceName, opts...)\n")
	b.WriteString("\tif err != nil { return nil, err }\n")
	b.WriteString("\treturn NewForConfig(cfg)\n")
	b.WriteString("}\n")
}

func clientsetImports(s *spec.Spec) string {
	var b strings.Builder
	b.WriteString("import (\n")
	b.WriteString("\tfmt \"fmt\"\n")
	b.WriteString("\thttp \"net/http\"\n\n")
	b.WriteString("\tdiscovery \"k8s.io/client-go/discovery\"\n")
	b.WriteString("\trest \"k8s.io/client-go/rest\"\n")
	b.WriteString("\tflowcontrol \"k8s.io/client-go/util/flowcontrol\"\n\n")
	b.WriteString("\tmeshrest \"github.com/onexstack/onexmesh/pkg/client/rest\"\n\n")
	for _, g := range s.Groups {
		for _, vs := range g.Versions {
			fmt.Fprintf(&b, "\t%s %q\n", typedAlias(g, vs), typedPackagePath(s, g, vs))
		}
	}
	b.WriteString(")\n\n")
	return b.String()
}

// typedAlias returns the import alias of a typed group/version package, e.g.
// "appsv1".
func typedAlias(g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return strings.ToLower(g.GroupGoName) + vs.Version
}

// groupVersionGoName returns the Go name of the group/version accessor, e.g.
// "AppsV1".
func groupVersionGoName(g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return g.GroupGoName + upperFirst(vs.Version)
}

// typedPackagePath returns the import path of a typed group/version package.
func typedPackagePath(s *spec.Spec, g *spec.GroupSpec, vs *spec.VersionSpec) string {
	return s.ClientsetGoPackage + "/typed/" + groupPkgName(g) + "/" + vs.Version
}

func groupPkgName(g *spec.GroupSpec) string {
	if g.Group == "" {
		return "core"
	}
	return g.Group
}
