// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/client"
	"github.com/onexstack/onexmesh/pkg/selector"
)

var _ IOptions = (*SelectorOptions)(nil)

// SelectorOptions selects the load-balancing strategy and client-side discovery
// caching.
type SelectorOptions struct {
	Strategy string // round_robin, random, weighted, p2c
	// DiscoveryCacheTTL enables the read-through discovery cache when > 0.
	DiscoveryCacheTTL time.Duration
}

// NewSelectorOptions returns default selector options.
func NewSelectorOptions() *SelectorOptions {
	return &SelectorOptions{Strategy: "round_robin"}
}

func (o *SelectorOptions) Validate() []error {
	if selector.Registered(o.Strategy) {
		return nil
	}
	return []error{fmt.Errorf("invalid selector strategy %q", o.Strategy)}
}

func (o *SelectorOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&o.Strategy, prefix+".strategy", o.Strategy, "Load-balancing strategy: round_robin, random, weighted, p2c.")
	fs.DurationVar(&o.DiscoveryCacheTTL, prefix+".discovery-cache-ttl", o.DiscoveryCacheTTL, "Discovery read-through cache TTL (0 disables).")
}

// DialOption converts the strategy selection into a client DialOption.
func (o *SelectorOptions) DialOption() client.DialOption {
	return client.WithSelector(o.Strategy)
}

var _ IOptions = (*ResilienceOptions)(nil)

// ResilienceOptions configures client-side resilience.
type ResilienceOptions struct {
	MaxAttempts          int
	BaseBackoff          time.Duration
	MaxBackoff           time.Duration
	BreakerWindow        time.Duration
	BreakerProbeInterval time.Duration
	Timeout              time.Duration
	// Bulkhead bounds the number of concurrent in-flight requests to a single
	// downstream service. 0 disables the bulkhead.
	Bulkhead int
	// PolicyPath enables declarative resilience: one or more resiliency YAML
	// files whose named policies (timeout/retry/breaker) are bound to endpoints.
	// When set, declarative policies take precedence over the imperative fields.
	PolicyPath []string
}

// NewResilienceOptions returns default resilience options.
func NewResilienceOptions() *ResilienceOptions {
	return &ResilienceOptions{
		MaxAttempts:          3,
		BaseBackoff:          100 * time.Millisecond,
		MaxBackoff:           time.Second,
		BreakerWindow:        10 * time.Second,
		BreakerProbeInterval: time.Second,
		Timeout:              5 * time.Second,
	}
}

func (o *ResilienceOptions) Validate() []error {
	var errs []error
	if o.MaxAttempts < 0 {
		errs = append(errs, fmt.Errorf("resilience max-attempts must be >= 0, got %d", o.MaxAttempts))
	}
	if o.BaseBackoff < 0 {
		errs = append(errs, fmt.Errorf("resilience base-backoff must be >= 0, got %v", o.BaseBackoff))
	}
	if o.MaxBackoff < 0 {
		errs = append(errs, fmt.Errorf("resilience max-backoff must be >= 0, got %v", o.MaxBackoff))
	}
	if o.MaxBackoff < o.BaseBackoff {
		errs = append(errs, fmt.Errorf("resilience max-backoff (%v) must be >= base-backoff (%v)", o.MaxBackoff, o.BaseBackoff))
	}
	if o.BreakerWindow < 0 {
		errs = append(errs, fmt.Errorf("resilience breaker-window must be >= 0, got %v", o.BreakerWindow))
	}
	if o.BreakerProbeInterval < 0 {
		errs = append(errs, fmt.Errorf("resilience breaker-probe-interval must be >= 0, got %v", o.BreakerProbeInterval))
	}
	if o.Timeout < 0 {
		errs = append(errs, fmt.Errorf("resilience timeout must be >= 0, got %v", o.Timeout))
	}
	if o.Bulkhead < 0 {
		errs = append(errs, fmt.Errorf("resilience bulkhead must be >= 0, got %d", o.Bulkhead))
	}
	return errs
}

func (o *ResilienceOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.IntVar(&o.MaxAttempts, prefix+".max-attempts", o.MaxAttempts, "Max total attempts for retry.")
	fs.DurationVar(&o.BaseBackoff, prefix+".base-backoff", o.BaseBackoff, "Retry base backoff.")
	fs.DurationVar(&o.MaxBackoff, prefix+".max-backoff", o.MaxBackoff, "Retry max backoff.")
	fs.DurationVar(&o.BreakerWindow, prefix+".breaker-window", o.BreakerWindow, "Breaker sliding window duration.")
	fs.DurationVar(&o.BreakerProbeInterval, prefix+".breaker-probe-interval", o.BreakerProbeInterval, "Breaker force-pass interval.")
	fs.DurationVar(&o.Timeout, prefix+".timeout", o.Timeout, "Per-call timeout.")
	fs.IntVar(&o.Bulkhead, prefix+".bulkhead", o.Bulkhead, "Max concurrent in-flight requests per downstream (0 disables).")
	fs.StringSliceVar(&o.PolicyPath, prefix+".policy-path", o.PolicyPath, "Declarative resiliency policy YAML files.")
}

// DialOptions converts the resilience settings into client DialOptions.
func (o *ResilienceOptions) DialOptions() []client.DialOption {
	var opts []client.DialOption
	if o.MaxAttempts > 1 {
		opts = append(opts, client.WithRetry(o.MaxAttempts, o.BaseBackoff, o.MaxBackoff))
	}
	if o.BreakerWindow > 0 {
		opts = append(opts, client.WithBreaker(o.BreakerWindow, o.BreakerProbeInterval))
	}
	if o.Bulkhead > 0 {
		opts = append(opts, client.WithBulkhead(o.Bulkhead))
	}
	if o.Timeout > 0 {
		opts = append(opts, client.WithTimeout(o.Timeout))
	}
	return opts
}
