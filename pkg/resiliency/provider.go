// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resiliency

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"

	"github.com/onexstack/onexstack/pkg/errorsx"
)

// Provider resolves resilience policies for service endpoints.
type Provider interface {
	// EndpointPolicy returns the compiled policy for an app's endpoint, or nil
	// when no policy is defined for it.
	EndpointPolicy(app, endpoint string) *PolicyDefinition
	// PolicyDefined reports whether the app has any resilience configuration.
	PolicyDefined(app string) bool
}

type appPolicies struct {
	timeouts map[string]time.Duration
	retries  map[string]*RetryPolicy
	breakers map[string]*breakerTemplate

	mu      sync.Mutex
	targets map[string]*EndpointPolicyNames
	cbCache map[string]CircuitBreakerState
	cbMax   int
}

type breakerTemplate struct {
	maxRequests int
	interval    time.Duration
	timeout     time.Duration
}

type provider struct {
	apps map[string]*appPolicies
}

// FromConfigurations builds a Provider from parsed Resiliency configurations.
func FromConfigurations(cfgs ...*Resiliency) (Provider, error) {
	p := &provider{apps: make(map[string]*appPolicies)}
	for _, cfg := range cfgs {
		if cfg == nil {
			continue
		}
		ap, err := buildAppPolicies(cfg)
		if err != nil {
			return nil, err
		}
		for app := range cfg.Spec.Targets.Apps {
			p.apps[app] = ap
		}
		if cfg.Name != "" {
			p.apps[cfg.Name] = ap
		}
	}
	return p, nil
}

// Load reads resiliency YAML files and builds a Provider. Each file is a single
// Resiliency document.
func Load(paths ...string) (Provider, error) {
	var cfgs []*Resiliency
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("resiliency: read %s: %w", path, err)
		}
		var cfg Resiliency
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("resiliency: parse %s: %w", path, err)
		}
		cfgs = append(cfgs, &cfg)
	}
	return FromConfigurations(cfgs...)
}

func (p *provider) EndpointPolicy(app, endpoint string) *PolicyDefinition {
	ap, ok := p.apps[app]
	if !ok {
		return nil
	}
	ap.mu.Lock()
	names, ok := ap.targets[app]
	ap.mu.Unlock()
	if !ok {
		return nil
	}
	return ap.buildDefinition(endpoint, names)
}

func (p *provider) PolicyDefined(app string) bool {
	_, ok := p.apps[app]
	return ok
}

func buildAppPolicies(cfg *Resiliency) (*appPolicies, error) {
	ap := &appPolicies{
		timeouts: make(map[string]time.Duration),
		retries:  make(map[string]*RetryPolicy),
		breakers: make(map[string]*breakerTemplate),
		targets:  make(map[string]*EndpointPolicyNames),
		cbCache:  make(map[string]CircuitBreakerState),
		cbMax:    100,
	}

	for name, d := range cfg.Spec.Policies.Timeouts {
		parsed, err := time.ParseDuration(d)
		if err != nil {
			return nil, fmt.Errorf("resiliency: timeout %q: %w", name, err)
		}
		ap.timeouts[name] = parsed
	}

	for name, r := range cfg.Spec.Policies.Retries {
		rp, err := buildRetryPolicy(r)
		if err != nil {
			return nil, fmt.Errorf("resiliency: retry %q: %w", name, err)
		}
		ap.retries[name] = rp
	}

	for name, cb := range cfg.Spec.Policies.CircuitBreakers {
		timeout, err := parseOptionalDuration(cb.Timeout)
		if err != nil {
			return nil, fmt.Errorf("resiliency: circuit breaker %q: %w", name, err)
		}
		interval, err := parseOptionalDuration(cb.Interval)
		if err != nil {
			return nil, fmt.Errorf("resiliency: circuit breaker %q: %w", name, err)
		}
		ap.breakers[name] = &breakerTemplate{
			maxRequests: cb.MaxRequests,
			interval:    interval,
			timeout:     timeout,
		}
	}

	for app, names := range cfg.Spec.Targets.Apps {
		n := names
		ap.targets[app] = &n
		if names.CircuitBreakerCacheSize > 0 {
			ap.cbMax = names.CircuitBreakerCacheSize
		}
	}
	return ap, nil
}

func (ap *appPolicies) buildDefinition(endpoint string, names *EndpointPolicyNames) *PolicyDefinition {
	def := &PolicyDefinition{Name: endpoint}

	if d, ok := ap.timeouts[names.Timeout]; ok {
		def.Timeout = d
	}
	if rp, ok := ap.retries[names.Retry]; ok {
		def.Retry = rp
	}
	if tpl, ok := ap.breakers[names.CircuitBreaker]; ok {
		def.CircuitBreaker = ap.endpointBreaker(endpoint, tpl)
	}
	return def
}

// endpointBreaker returns a per-endpoint breaker, creating and caching it so
// the breaker state survives across policy lookups.
func (ap *appPolicies) endpointBreaker(endpoint string, tpl *breakerTemplate) CircuitBreakerState {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	if cb, ok := ap.cbCache[endpoint]; ok {
		return cb
	}
	cb := newBreaker(breakerOptions{
		maxRequests: tpl.maxRequests,
		interval:    tpl.interval,
		timeout:     tpl.timeout,
	})
	if len(ap.cbCache) < ap.cbMax {
		ap.cbCache[endpoint] = cb
	}
	return cb
}

func buildRetryPolicy(r Retry) (*RetryPolicy, error) {
	rp := &RetryPolicy{policy: r.Policy}
	if rp.policy == "" {
		rp.policy = "constant"
	}

	d, err := parseOptionalDuration(r.Duration)
	if err != nil {
		return nil, err
	}
	if d <= 0 {
		d = time.Second
	}
	rp.duration = d

	rp.maxInterval, err = parseOptionalDuration(r.MaxInterval)
	if err != nil {
		return nil, err
	}

	rp.maxRetries = 3
	if r.MaxRetries != nil {
		rp.maxRetries = *r.MaxRetries
	}

	matching, err := buildMatching(r.Matching)
	if err != nil {
		return nil, err
	}
	rp.matching = matching
	return rp, nil
}

func buildMatching(m *RetryMatching) (func(error) bool, error) {
	if m == nil {
		return nil, nil // nil means "retry all errors"
	}
	httpMatch, err := ParseStatusRanges(m.HTTPStatusCodes)
	if err != nil {
		return nil, err
	}
	grpcMatch, err := ParseStatusRanges(m.GRPCStatusCodes)
	if err != nil {
		return nil, err
	}

	hasHTTP := strings.TrimSpace(m.HTTPStatusCodes) != ""
	hasGRPC := strings.TrimSpace(m.GRPCStatusCodes) != ""

	return func(err error) bool {
		if err == nil {
			return false
		}
		if hasHTTP && httpMatch(errorsx.Code(err)) {
			return true
		}
		if hasGRPC {
			if code := int(status.Code(err)); code >= 0 && grpcMatch(code) {
				return true
			}
		}
		// With no status-code matching configured, retry everything.
		return !hasHTTP && !hasGRPC
	}, nil
}

func parseOptionalDuration(s string) (time.Duration, error) {
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}
