// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resiliency

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestParseStatusRanges(t *testing.T) {
	m, err := ParseStatusRanges("500,502-504")
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{500, 502, 503, 504} {
		if !m(code) {
			t.Errorf("match(%d) = false, want true", code)
		}
	}
	for _, code := range []int{400, 501, 505} {
		if m(code) {
			t.Errorf("match(%d) = true, want false", code)
		}
	}
}

func TestBreakerOpensOnConsecutiveFailures(t *testing.T) {
	b := &breaker{state: StateClosed, maxRequests: 1, timeout: time.Hour, threshold: 2}
	fail := func() error { return errors.New("boom") }

	_ = b.Execute(fail, nil)
	_ = b.Execute(fail, nil)

	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	if err := b.Execute(func() error { return nil }, nil); err != ErrCircuitOpen {
		t.Fatalf("err = %v, want ErrCircuitOpen", err)
	}
}

func TestBreakerHalfOpenRecovers(t *testing.T) {
	b := &breaker{state: StateOpen, maxRequests: 1, timeout: 10 * time.Millisecond, threshold: 2, openedAt: time.Now()}

	time.Sleep(20 * time.Millisecond)
	if err := b.Execute(func() error { return nil }, nil); err != nil {
		t.Fatalf("half-open probe err = %v, want nil", err)
	}
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed after success", b.State())
	}
}

func TestRunnerRetriesUntilSuccess(t *testing.T) {
	def := &PolicyDefinition{
		Retry: &RetryPolicy{policy: "constant", duration: time.Millisecond, maxRetries: 2},
	}

	attempts := 0
	_, err := Runner(context.Background(), def, func(context.Context) (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errors.New("transient")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("Runner() err = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRunnerTimeout(t *testing.T) {
	def := &PolicyDefinition{Timeout: 10 * time.Millisecond}

	_, err := Runner(context.Background(), def, func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
}

func TestProviderEndpointPolicy(t *testing.T) {
	maxRetries := 2
	cfg := &Resiliency{
		Name: "svc",
		Spec: Spec{
			Policies: Policies{
				Timeouts: map[string]string{"fast": "100ms"},
				Retries:  map[string]Retry{"r": {Duration: "10ms", MaxRetries: &maxRetries}},
			},
			Targets: Targets{
				Apps: map[string]EndpointPolicyNames{
					"svc": {Timeout: "fast", Retry: "r"},
				},
			},
		},
	}

	p, err := FromConfigurations(cfg)
	if err != nil {
		t.Fatal(err)
	}

	def := p.EndpointPolicy("svc", "/svc/Get")
	if def == nil {
		t.Fatal("EndpointPolicy() = nil, want a policy")
	}
	if def.Timeout != 100*time.Millisecond {
		t.Fatalf("Timeout = %v, want 100ms", def.Timeout)
	}
	if def.Retry == nil || def.Retry.maxRetries != 2 {
		t.Fatalf("Retry = %+v, want maxRetries 2", def.Retry)
	}
}

func TestProviderNoPolicy(t *testing.T) {
	p, _ := FromConfigurations(&Resiliency{Name: "svc"})
	if def := p.EndpointPolicy("svc", "/svc/Get"); def != nil {
		t.Fatalf("EndpointPolicy() = %+v, want nil", def)
	}
	if !p.PolicyDefined("svc") {
		t.Fatal("PolicyDefined(svc) = false, want true")
	}
}
