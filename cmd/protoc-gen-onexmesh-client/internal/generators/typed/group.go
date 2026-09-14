// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package typed

import (
	"fmt"
	"strings"
)

// GenerateGroup renders the per-group/version client shell (apps_client.go).
func GenerateGroup(tc *TypeContext) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "package %s\n\n", typedPackageName(tc))
	b.WriteString(groupImports(tc))

	// Group interface.
	fmt.Fprintf(&b, "// %s has methods to work with %s resources in a given namespace.\n", tc.GroupInterface, tc.Group.Group)
	fmt.Fprintf(&b, "type %s interface {\n", tc.GroupInterface)
	b.WriteString("\tRESTClient() rest.Interface\n")
	for _, rs := range tc.Version.Resources {
		plural := pluralGoName(rs)
		fmt.Fprintf(&b, "\t%sGetter\n", plural)
	}
	b.WriteString("}\n\n")

	// Group client.
	fmt.Fprintf(&b, "// %s implements %s.\n", tc.GroupClient, tc.GroupInterface)
	fmt.Fprintf(&b, "type %s struct {\n", tc.GroupClient)
	b.WriteString("\trestClient rest.Interface\n")
	b.WriteString("}\n\n")

	for _, rs := range tc.Version.Resources {
		plural := pluralGoName(rs)
		if rs.Namespaced {
			fmt.Fprintf(&b, "func (c *%s) %s(namespace string) %sInterface {\n", tc.GroupClient, plural, rs.Kind)
			fmt.Fprintf(&b, "\treturn new%s(c, namespace)\n", plural)
		} else {
			fmt.Fprintf(&b, "func (c *%s) %s() %sInterface {\n", tc.GroupClient, plural, rs.Kind)
			fmt.Fprintf(&b, "\treturn new%s(c)\n", plural)
		}
		b.WriteString("}\n\n")
	}

	// RESTClient accessor.
	b.WriteString("func (c *" + tc.GroupClient + ") RESTClient() rest.Interface {\n")
	b.WriteString("\tif c == nil { return nil }\n")
	b.WriteString("\treturn c.restClient\n")
	b.WriteString("}\n\n")

	generateGroupConstructors(&b, tc)

	return b.String()
}

func generateGroupConstructors(b *strings.Builder, tc *TypeContext) {
	// NewForConfig.
	fmt.Fprintf(b, "func NewForConfig(c *rest.Config) (*%s, error) {\n", tc.GroupClient)
	b.WriteString("\tconfig := *c\n")
	b.WriteString("\tif err := setConfigDefaults(&config); err != nil { return nil, err }\n")
	b.WriteString("\thttpClient, err := rest.HTTPClientFor(&config)\n")
	b.WriteString("\tif err != nil { return nil, err }\n")
	b.WriteString("\treturn NewForConfigAndClient(&config, httpClient)\n")
	b.WriteString("}\n\n")

	// NewForConfigAndClient.
	fmt.Fprintf(b, "func NewForConfigAndClient(c *rest.Config, h *http.Client) (*%s, error) {\n", tc.GroupClient)
	b.WriteString("\tconfig := *c\n")
	b.WriteString("\tif err := setConfigDefaults(&config); err != nil { return nil, err }\n")
	b.WriteString("\tclient, err := rest.RESTClientForConfigAndClient(&config, h)\n")
	b.WriteString("\tif err != nil { return nil, err }\n")
	fmt.Fprintf(b, "\treturn &%s{client}, nil\n", tc.GroupClient)
	b.WriteString("}\n\n")

	// NewForConfigOrDie.
	fmt.Fprintf(b, "func NewForConfigOrDie(c *rest.Config) *%s {\n", tc.GroupClient)
	fmt.Fprintf(b, "\tclient, err := NewForConfig(c)\n")
	b.WriteString("\tif err != nil { panic(err) }\n")
	b.WriteString("\treturn client\n")
	b.WriteString("}\n\n")

	// New.
	fmt.Fprintf(b, "func New(c rest.Interface) *%s {\n", tc.GroupClient)
	fmt.Fprintf(b, "\treturn &%s{c}\n", tc.GroupClient)
	b.WriteString("}\n\n")

	// setConfigDefaults.
	b.WriteString("func setConfigDefaults(config *rest.Config) error {\n")
	fmt.Fprintf(b, "\tgv := %s.SchemeGroupVersion\n", tc.APIAlias)
	b.WriteString("\tconfig.GroupVersion = &gv\n")
	fmt.Fprintf(b, "\tconfig.APIPath = %q\n", apiPath(tc))
	b.WriteString("\tif config.NegotiatedSerializer == nil {\n")
	b.WriteString("\t\tconfig.NegotiatedSerializer = rest.CodecFactoryForGeneratedClient(scheme.Scheme, scheme.Codecs).WithoutConversion()\n")
	b.WriteString("\t}\n")
	b.WriteString("\tif config.UserAgent == \"\" {\n")
	b.WriteString("\t\tconfig.UserAgent = rest.DefaultKubernetesUserAgent()\n")
	b.WriteString("\t}\n")
	b.WriteString("\treturn nil\n")
	b.WriteString("}\n")
}

// apiPath returns the API path for a group: the core group (empty group) is
// served at "/api", every other group at the clientset's API path (default
// "/apis"), mirroring client-gen's behavior.
func apiPath(tc *TypeContext) string {
	if tc.Group.Group == "" {
		return "/api"
	}
	return tc.Spec.ClientsetAPIPath
}

func groupImports(tc *TypeContext) string {
	var b strings.Builder
	b.WriteString("import (\n")
	b.WriteString("\thttp \"net/http\"\n\n")
	fmt.Fprintf(&b, "\t%s %q\n", tc.APIAlias, tc.Version.GoPackage)
	fmt.Fprintf(&b, "\tscheme %q\n", tc.Spec.ClientsetGoPackage+"/scheme")
	b.WriteString("\trest \"k8s.io/client-go/rest\"\n")
	b.WriteString(")\n\n")
	return b.String()
}
