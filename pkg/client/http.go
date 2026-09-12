// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onexstack/onexmesh/pkg/codec"
	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/registry/cache"
	"github.com/onexstack/onexmesh/pkg/selector"
)

// httpOptions holds the options for an HTTP service-discovery client.
type httpOptions struct {
	registryName string
	registryOpts any
	discovery    registry.Discovery
	selector     selector.Selector
	timeout      time.Duration
	httpClient   *http.Client
	codec        codec.Marshaler
	resilience   resilienceConfig
	cacheTTL     time.Duration
	cacheOpts    []cache.Option
}

// HTTPOption configures an HTTPClient.
type HTTPOption func(*httpOptions)

// WithHTTPRegistry selects the registry backend and its options.
func WithHTTPRegistry(name string, opts any) HTTPOption {
	return func(o *httpOptions) {
		o.registryName = name
		o.registryOpts = opts
	}
}

// WithHTTPDiscovery injects a pre-built registry.Discovery, taking precedence
// over WithHTTPRegistry, mirroring WithDiscovery for the gRPC client.
func WithHTTPDiscovery(d registry.Discovery) HTTPOption {
	return func(o *httpOptions) { o.discovery = d }
}

// WithHTTPSelector overrides the default round-robin selector.
func WithHTTPSelector(s selector.Selector) HTTPOption {
	return func(o *httpOptions) { o.selector = s }
}

// WithHTTPTimeout sets the HTTP client timeout.
func WithHTTPTimeout(d time.Duration) HTTPOption {
	return func(o *httpOptions) { o.timeout = d }
}

// WithHTTPClient injects a pre-configured *http.Client.
func WithHTTPClient(c *http.Client) HTTPOption {
	return func(o *httpOptions) { o.httpClient = c }
}

// WithHTTPCodec sets the request/response body codec, defaulting to JSON.
func WithHTTPCodec(c codec.Marshaler) HTTPOption {
	return func(o *httpOptions) { o.codec = c }
}

// WithHTTPRetry enables retry with exponential backoff and jitter.
func WithHTTPRetry(maxAttempts int, base, max time.Duration) HTTPOption {
	return func(o *httpOptions) {
		o.resilience.maxAttempts = maxAttempts
		o.resilience.baseBackoff = base
		o.resilience.maxBackoff = max
	}
}

// WithHTTPRetryable sets the predicate deciding which errors are retried.
func WithHTTPRetryable(f func(error) bool) HTTPOption {
	return func(o *httpOptions) { o.resilience.retryable = f }
}

// WithHTTPBreaker enables a Google SRE sliding-window circuit breaker around
// HTTP calls.
func WithHTTPBreaker(window, probeInterval time.Duration) HTTPOption {
	return func(o *httpOptions) {
		o.resilience.breakerWindow = window
		o.resilience.breakerProbe = probeInterval
	}
}

// WithHTTPBulkhead isolates the downstream service by bounding the number of
// concurrent in-flight HTTP requests, mirroring WithBulkhead for the gRPC client.
// A value <= 0 disables the bulkhead.
func WithHTTPBulkhead(maxConcurrent int) HTTPOption {
	return func(o *httpOptions) { o.resilience.maxConcurrent = maxConcurrent }
}

// WithHTTPDiscoveryCache wraps the discovery backend in a read-through cache
// with the given service TTL, mirroring WithDiscoveryCache for the gRPC client.
// It enables singleflight deduplication and stale-while-error degradation. When
// ttl <= 0 the cache is disabled.
func WithHTTPDiscoveryCache(ttl time.Duration, opts ...cache.Option) HTTPOption {
	return func(o *httpOptions) {
		o.cacheTTL = ttl
		o.cacheOpts = opts
	}
}

// HTTPClient performs HTTP calls against a service resolved via the registry.
// It maintains a local cache of discovered nodes, refreshed by a background
// watch, so per-request calls do not hit the registry directly.
type HTTPClient struct {
	serviceName string
	discovery   registry.Discovery
	selector    selector.Selector
	httpClient  *http.Client
	codec       codec.Marshaler
	resilience  resilienceConfig

	nodes     atomic.Value // []selector.Node
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// defaultHTTPTransport is a shared, pooled transport used by HTTPClients that
// do not inject their own http.Client. It clones http.DefaultTransport and
// raises the per-host idle-connection limit so service-to-service calls reuse
// connections instead of paying a fresh handshake per request.
var defaultHTTPTransport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxIdleConnsPerHost = 20
	return t
}()

// NewHTTPClient creates an HTTP service-discovery client.
func NewHTTPClient(serviceName string, opts ...HTTPOption) (*HTTPClient, error) {
	o := &httpOptions{}
	for _, opt := range opts {
		opt(o)
	}

	discovery := o.discovery
	if discovery == nil {
		if o.registryName == "" {
			return nil, fmt.Errorf("client: registry not configured; use WithHTTPRegistry or WithHTTPDiscovery")
		}
		d, err := registry.CreateDiscovery(o.registryName, o.registryOpts)
		if err != nil {
			return nil, err
		}
		discovery = d
	}
	if o.cacheTTL > 0 {
		discovery = cache.New(discovery, append([]cache.Option{cache.WithTTL(o.cacheTTL)}, o.cacheOpts...)...)
	}

	var err error
	sel := o.selector
	if sel == nil {
		sel, err = selector.GetSelector("round_robin")
		if err != nil {
			return nil, err
		}
	}

	hc := o.httpClient
	if hc == nil {
		hc = &http.Client{Timeout: o.timeout, Transport: defaultHTTPTransport}
	}

	mc := o.codec
	if mc == nil {
		mc = codec.JSON{}
	}

	c := &HTTPClient{
		serviceName: serviceName,
		discovery:   discovery,
		selector:    sel,
		httpClient:  hc,
		codec:       mc,
		resilience:  o.resilience,
	}
	c.startWatch()
	return c, nil
}

// startWatch launches a background goroutine that seeds the node cache from the
// registry and then keeps it fresh via Watch. The goroutine exits when Close is
// called or the discovery backend's watcher terminates.
func (c *HTTPClient) startWatch() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go c.watch(ctx)
}

func (c *HTTPClient) watch(ctx context.Context) {
	instances, err := c.discovery.GetService(ctx, c.serviceName)
	if err == nil {
		c.nodes.Store(instancesToNodes(c.serviceName, instances))
	}

	watcher, err := c.discovery.Watch(ctx, c.serviceName)
	if err != nil {
		return
	}
	defer watcher.Stop()

	for {
		instances, err := watcher.Next()
		if err != nil {
			return
		}
		c.nodes.Store(instancesToNodes(c.serviceName, instances))
	}
}

// Close stops the background watch and releases its resources. It is idempotent.
func (c *HTTPClient) Close() {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
	})
}

// Do resolves a backend from the cached nodes, encodes req with the configured
// codec, performs the request and decodes the response into resp. When
// resilience is configured the select+request step is wrapped with
// retry/breaker/timeout.
func (c *HTTPClient) Do(ctx context.Context, method, path string, req, resp interface{}) error {
	nodes, _ := c.nodes.Load().([]selector.Node)
	if nodes == nil {
		// Fall back to synchronous discovery on the very first call, before the
		// background watch has seeded the cache.
		instances, err := c.discovery.GetService(ctx, c.serviceName)
		if err != nil {
			return err
		}
		nodes = instancesToNodes(c.serviceName, instances)
	}
	if len(nodes) == 0 {
		return fmt.Errorf("client: no instances for %q", c.serviceName)
	}

	doOnce := func(ctx context.Context) error {
		node, done, err := c.selector.Select(ctx, nodes)
		if err != nil {
			return err
		}
		err = c.do(ctx, node, method, path, req, resp)
		if done != nil {
			done(ctx, selector.DoneInfo{Err: err})
		}
		return err
	}

	if c.resilience.enabled() {
		return buildResilienceHandler(c.resilience, doOnce)(ctx)
	}
	return doOnce(ctx)
}

// do performs a single HTTP request against the selected node.
func (c *HTTPClient) do(ctx context.Context, node selector.Node, method, path string, req, resp interface{}) error {
	scheme := node.Scheme()
	if scheme == "" {
		// A scheme-less endpoint is assumed to be plain HTTP.
		scheme = "http"
	}
	url := fmt.Sprintf("%s://%s%s", scheme, node.Address(), path)

	var body io.Reader
	if req != nil {
		b, err := c.codec.Marshal(req)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if req != nil {
		httpReq.Header.Set("Content-Type", c.codec.ContentType())
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("client: unexpected status %d", httpResp.StatusCode)
	}
	if resp != nil {
		data, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return err
		}
		return c.codec.Unmarshal(data, resp)
	}
	return nil
}

// instancesToNodes converts discovered instances into selector nodes, keeping
// only http:// (or scheme-less) endpoints so the HTTP client never picks a
// grpc:// endpoint.
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

// parseSchemeHost parses a "scheme://host:port" endpoint into scheme and host:port.
func parseSchemeHost(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", err
	}
	if u.Host == "" {
		return "", "", fmt.Errorf("endpoint %q has no host", endpoint)
	}
	return u.Scheme, u.Host, nil
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
