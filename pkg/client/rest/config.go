// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package rest

import (
	"net/http"

	"k8s.io/client-go/rest"

	meshclient "github.com/onexstack/onexmesh/pkg/client"
	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/registry/cache"
	"github.com/onexstack/onexmesh/pkg/selector"
)

// placeholderHost is the rest.Config.Host used by NewForMeshConfig. It is
// deliberately a valid http:// URL so client-go can parse it into a base URL;
// the meshRoundTripper rewrites the actual scheme/host per request, so this
// value never reaches the wire.
const placeholderHost = "http://onexmesh.invalid"

// NewForMeshConfig builds a client-go rest.Config whose transport discovers the
// logical service via the onexmesh registry and load-balances across its
// instances. The returned config is meant to be passed to a generated
// clientset's NewForConfig.
//
// The caller owns any TLS configuration: for TLS-backed (https) instances set
// the returned config's TLSClientConfig (e.g. Insecure or CAData) before use.
func NewForMeshConfig(serviceName string, opts ...Option) (*rest.Config, error) {
	rt, err := newMeshRoundTripper(serviceName, opts...)
	if err != nil {
		return nil, err
	}

	return &rest.Config{
		Host: placeholderHost,
		// WrapTransport is invoked by client-go after it builds the base
		// transport (TLS/keepalive/timeouts applied); we retain it and rewrite
		// scheme/host on every request.
		WrapTransport: func(base http.RoundTripper) http.RoundTripper {
			rt.base = base
			return rt
		},
	}, nil
}

// newMeshRoundTripper resolves the discovery/selector/resilience from opts and
// returns a meshRoundTripper whose base transport is injected later via
// WrapTransport.
func newMeshRoundTripper(serviceName string, opts ...Option) (*meshRoundTripper, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	discovery := o.discovery
	if discovery == nil {
		name, ropts := o.registryName, o.registryOpts
		if name == "" {
			name, ropts = meshclient.DefaultRegistry()
		}
		d, err := registry.CreateDiscovery(name, ropts)
		if err != nil {
			return nil, err
		}
		discovery = d
	}
	if o.cacheTTL > 0 {
		discovery = cache.New(discovery, append([]cache.Option{cache.WithTTL(o.cacheTTL)}, o.cacheOpts...)...)
	}

	sel := o.selector
	if sel == nil {
		var err error
		sel, err = selector.GetSelector("round_robin")
		if err != nil {
			return nil, err
		}
	}

	return &meshRoundTripper{
		serviceName: serviceName,
		discovery:   discovery,
		selector:    sel,
		resilience:  buildResilience(o),
	}, nil
}
