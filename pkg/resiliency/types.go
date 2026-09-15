// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package resiliency provides declarative, configuration-driven resilience
// policies (timeouts, retries and circuit breakers) bound to service endpoints,
// modeled after Dapr's resiliency policy model. It composes with the primitive
// middleware in pkg/resilience but adds named-policy templates, target binding,
// status-code-range matching and per-endpoint breaker state.
//
// Relationship to pkg/resilience: pkg/resilience is the imperative layer of
// client-side primitives (circuit breaker, retry, timeout, hedge, shedder,
// deadline, bulkhead) written as func(Handler) Handler middleware. This package
// is the declarative layer above it: a YAML-driven Provider resolves a
// PolicyDefinition per app endpoint at call time. Note that the circuit breaker
// implemented in breaker.go is a "consecutive-failure threshold" breaker, which
// differs in semantics from the Google SRE sliding-window breaker in
// pkg/resilience — the two are intentionally distinct algorithms, not a
// duplicated implementation.
package resiliency

// Resiliency is a named resiliency configuration.
type Resiliency struct {
	Name   string   `json:"name" yaml:"name"`
	Spec   Spec     `json:"spec" yaml:"spec"`
	Scopes []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

// Spec holds the policy templates and their target bindings.
type Spec struct {
	Policies Policies `json:"policies" yaml:"policies"`
	Targets  Targets  `json:"targets" yaml:"targets"`
}

// Policies holds named policy templates.
type Policies struct {
	Timeouts        map[string]string         `json:"timeouts,omitempty" yaml:"timeouts,omitempty"`
	Retries         map[string]Retry          `json:"retries,omitempty" yaml:"retries,omitempty"`
	CircuitBreakers map[string]CircuitBreaker `json:"circuitBreakers,omitempty" yaml:"circuitBreakers,omitempty"`
}

// Retry is a named retry policy template.
type Retry struct {
	// Policy is the backoff strategy: "constant" or "exponential".
	Policy string `json:"policy,omitempty" yaml:"policy,omitempty"`
	// Duration is the initial backoff duration, e.g. "100ms".
	Duration string `json:"duration,omitempty" yaml:"duration,omitempty"`
	// MaxInterval caps the backoff duration.
	MaxInterval string `json:"maxInterval,omitempty" yaml:"maxInterval,omitempty"`
	// MaxRetries is the number of retries after the first attempt.
	MaxRetries *int `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
	// Matching restricts which errors are retried by status code.
	Matching *RetryMatching `json:"matching,omitempty" yaml:"matching,omitempty"`
}

// RetryMatching selects retryable errors by status code range.
type RetryMatching struct {
	// HTTPStatusCodes is a comma/range list, e.g. "500,502-504".
	HTTPStatusCodes string `json:"httpStatusCodes,omitempty" yaml:"httpStatusCodes,omitempty"`
	// GRPCStatusCodes is a comma/range list of gRPC status codes, e.g. "14,8".
	GRPCStatusCodes string `json:"gRPCStatusCodes,omitempty" yaml:"gRPCStatusCodes,omitempty"`
}

// CircuitBreaker is a named circuit breaker policy template.
type CircuitBreaker struct {
	// MaxRequests is the number of requests allowed in the half-open state.
	MaxRequests int `json:"maxRequests,omitempty" yaml:"maxRequests,omitempty"`
	// Interval is reserved for a future sliding-window breaker. The current
	// breaker trips on a consecutive-failure threshold (see Trip), so this
	// field is parsed for compatibility but not yet applied.
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
	// Timeout is how long the breaker stays open before half-open.
	Timeout string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// Trip is a predicate describing when to open. A CEL expression is reserved
	// for future use; when empty, a default consecutive-failure threshold is
	// used.
	Trip string `json:"trip,omitempty" yaml:"trip,omitempty"`
}

// Targets binds named policies to endpoints.
type Targets struct {
	Apps map[string]EndpointPolicyNames `json:"apps,omitempty" yaml:"apps,omitempty"`
}

// EndpointPolicyNames references named policies for a single endpoint.
type EndpointPolicyNames struct {
	Timeout                 string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retry                   string `json:"retry,omitempty" yaml:"retry,omitempty"`
	CircuitBreaker          string `json:"circuitBreaker,omitempty" yaml:"circuitBreaker,omitempty"`
	CircuitBreakerCacheSize int    `json:"circuitBreakerCacheSize,omitempty" yaml:"circuitBreakerCacheSize,omitempty"`
}
