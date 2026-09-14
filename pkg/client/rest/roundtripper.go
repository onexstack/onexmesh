// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package rest provides a mesh-aware transport for client-go's rest.Config. It
// lets a generated clientset resolve a logical service name through the
// onexmesh registry and load-balance across its instances on every request,
// without giving up the typed client-go API surface.
package rest

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/resilience"
	"github.com/onexstack/onexmesh/pkg/selector"
)

// meshRoundTripper is an http.RoundTripper that rewrites each request's
// scheme/host to an instance discovered via the registry and selected via the
// selector, then delegates to the base transport.
//
// Unlike pkg/client.HTTPClient, it does not run a background watch: a
// roundtripper has no Close/Stop hook (client-go holds it through the lifetime
// of an http.Client), so a long-lived goroutine would leak. Instead it resolves
// nodes synchronously per request; callers that want to avoid hitting the
// registry on every request wrap the discovery in a cache (WithDiscoveryCache).
type meshRoundTripper struct {
	serviceName string
	discovery   registry.Discovery
	selector    selector.Selector
	base        http.RoundTripper
	resilience  resilience.Middleware
}

// RoundTrip selects an instance and forwards the request to it.
func (t *meshRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	nodes, err := t.nodes(req.Context())
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("rest: no instances for %q", t.serviceName)
	}

	var resp *http.Response
	doOnce := func(ctx context.Context) error {
		node, done, err := t.selector.Select(ctx, nodes)
		if err != nil {
			return err
		}
		r, err := t.forward(ctx, req, node)
		if done != nil {
			done(ctx, selector.DoneInfo{Err: err})
		}
		if err != nil {
			return err
		}
		resp = r
		return nil
	}

	var doErr error
	if t.resilience != nil {
		doErr = t.resilience(doOnce)(req.Context())
	} else {
		doErr = doOnce(req.Context())
	}
	if doErr != nil {
		return nil, doErr
	}
	return resp, nil
}

// nodes resolves the current instance set into selector nodes.
func (t *meshRoundTripper) nodes(ctx context.Context) ([]selector.Node, error) {
	instances, err := t.discovery.GetService(ctx, t.serviceName)
	if err != nil {
		return nil, err
	}
	return instancesToNodes(t.serviceName, instances), nil
}

// forward rewrites the request URL to the selected node and delegates to the
// base transport, preserving the request context for cancellation.
func (t *meshRoundTripper) forward(ctx context.Context, req *http.Request, node selector.Node) (*http.Response, error) {
	outReq := req.Clone(ctx)

	scheme := node.Scheme()
	if scheme == "" {
		scheme = "http"
	}
	outReq.URL.Scheme = scheme
	outReq.URL.Host = node.Address()
	// Drop the Host header so net/http re-derives it from outReq.URL.
	outReq.Host = ""

	return t.base.RoundTrip(outReq)
}

// buildResilience assembles the optional resilience chain, outermost first:
// deadline -> bulkhead -> breaker -> retry. A nil return disables resilience.
func buildResilience(o *options) resilience.Middleware {
	var mws []resilience.Middleware
	if o.timeout > 0 {
		mws = append(mws, resilience.Deadline(o.timeout))
	}
	if o.maxConcurrent > 0 {
		mws = append(mws, resilience.Bulkhead(o.maxConcurrent))
	}
	if o.breakerWindow > 0 {
		// Only transport/selection errors reach the breaker; success and HTTP
		// status errors (returned as a response) are acceptable.
		mws = append(mws, resilience.Breaker(
			resilience.WithWindow(o.breakerWindow),
			resilience.WithProbeInterval(o.breakerProbe),
			resilience.WithAcceptable(func(err error) bool { return err == nil }),
		))
	}
	if o.maxAttempts > 1 {
		retryable := o.retryable
		if retryable == nil {
			retryable = func(error) bool { return true }
		}
		mws = append(mws, resilience.Retry(
			resilience.WithMaxAttempts(o.maxAttempts),
			resilience.WithBackoff(o.baseBackoff, o.maxBackoff),
			resilience.WithRetryable(retryable),
		))
	}
	if len(mws) == 0 {
		return nil
	}
	return resilience.Chain(mws[0], mws[1:]...)
}

// instancesToNodes converts discovered instances into selector nodes, keeping
// only http(s) endpoints so the roundtripper never picks a grpc:// endpoint.
func instancesToNodes(serviceName string, instances []*registry.ServiceInstance) []selector.Node {
	var nodes []selector.Node
	for _, inst := range instances {
		for _, ep := range inst.Endpoints {
			scheme, hostport, err := parseSchemeHost(ep)
			if err != nil {
				continue
			}
			if scheme != "" && scheme != "http" && scheme != "https" {
				continue
			}
			nodes = append(nodes, selector.NewNode(hostport, scheme, serviceName, metadataWeight(inst.Metadata), inst.Metadata))
		}
	}
	return nodes
}

// parseSchemeHost parses a "scheme://host:port" endpoint into scheme and
// host:port.
func parseSchemeHost(endpoint string) (string, string, error) {
	scheme, rest, ok := strings.Cut(endpoint, "://")
	if !ok {
		return "", endpoint, nil
	}
	if scheme == "" || rest == "" {
		return "", "", fmt.Errorf("endpoint %q has no host", endpoint)
	}
	return scheme, rest, nil
}

// metadataWeight extracts an integer weight from instance metadata.
func metadataWeight(md map[string]string) int {
	if md == nil {
		return 100
	}
	if w, ok := md["weight"]; ok {
		if n, err := strconv.Atoi(w); err == nil {
			return n
		}
	}
	return 100
}
