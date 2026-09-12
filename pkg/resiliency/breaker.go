// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resiliency

import (
	"sync"
	"time"

	"github.com/onexstack/onexmesh/pkg/errno"
)

// ErrCircuitOpen is returned when the breaker rejects a request while open.
var ErrCircuitOpen = errno.ErrCircuitOpen

// State is a circuit breaker state.
type State int32

const (
	// StateClosed lets requests through normally.
	StateClosed State = iota
	// StateOpen rejects requests until the cooldown elapses.
	StateOpen
	// StateHalfOpen lets a bounded number of probe requests through.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerState protects requests with a closed/open/half-open breaker.
type CircuitBreakerState interface {
	// Execute runs req under the breaker, classifying results via acceptable
	// (nil acceptable treats any non-nil error as a failure).
	Execute(req func() error, acceptable func(error) bool) error
	// State reports the current breaker state.
	State() State
}

// breaker is a closed/open/half-open circuit breaker keyed on consecutive
// failures. The Trip CEL expression is reserved for a future richer predicate;
// until then a simple consecutive-failure threshold is used.
type breaker struct {
	mu sync.Mutex

	state       State
	maxRequests int
	timeout     time.Duration
	threshold   int

	consecutiveFailures int
	halfOpenRequests    int
	openedAt            time.Time
}

// defaultBreakerThreshold is the consecutive-failure count that opens a breaker
// when no Trip expression is configured.
const defaultBreakerThreshold = 5

type breakerOptions struct {
	maxRequests int
	interval    time.Duration
	timeout     time.Duration
}

func newBreaker(opts breakerOptions) *breaker {
	if opts.maxRequests <= 0 {
		opts.maxRequests = 1
	}
	if opts.timeout <= 0 {
		opts.timeout = 30 * time.Second
	}
	return &breaker{
		state:       StateClosed,
		maxRequests: opts.maxRequests,
		timeout:     opts.timeout,
		threshold:   defaultBreakerThreshold,
	}
}

func (b *breaker) Execute(req func() error, acceptable func(error) bool) error {
	if acceptable == nil {
		acceptable = func(err error) bool { return err == nil }
	}
	if err := b.beforeRequest(); err != nil {
		return err
	}
	err := req()
	b.afterRequest(err, acceptable)
	return err
}

func (b *breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *breaker) beforeRequest() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateOpen:
		if time.Since(b.openedAt) >= b.timeout {
			b.state = StateHalfOpen
			b.halfOpenRequests = 0
		} else {
			return ErrCircuitOpen
		}
	case StateHalfOpen:
		if b.halfOpenRequests >= b.maxRequests {
			return ErrCircuitOpen
		}
		b.halfOpenRequests++
	}
	return nil
}

func (b *breaker) afterRequest(err error, acceptable func(error) bool) {
	failed := !acceptable(err)

	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		if failed {
			b.consecutiveFailures++
			if b.consecutiveFailures >= b.threshold {
				b.state = StateOpen
				b.openedAt = time.Now()
			}
		} else {
			b.consecutiveFailures = 0
		}
	case StateHalfOpen:
		if failed {
			b.state = StateOpen
			b.openedAt = time.Now()
			b.consecutiveFailures = b.threshold
		} else {
			b.state = StateClosed
			b.consecutiveFailures = 0
			b.halfOpenRequests = 0
		}
	}
}
