// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onexstack/onexmesh/pkg/errno"
	"github.com/onexstack/onexmesh/pkg/resilience"
)

func TestTimeout(t *testing.T) {
	h := resilience.Timeout(10 * time.Millisecond)(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	err := h(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
}

func TestRetrySucceedsAfterFailures(t *testing.T) {
	attempts := 0
	h := resilience.Retry(resilience.WithBackoff(time.Millisecond, time.Millisecond))(
		func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("transient")
			}
			return nil
		})
	if err := h(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryNonRetryable(t *testing.T) {
	attempts := 0
	h := resilience.Retry(
		resilience.WithMaxAttempts(5),
		resilience.WithRetryable(func(err error) bool { return err.Error() == "retry-me" }),
	)(func(ctx context.Context) error {
		attempts++
		return errors.New("fatal")
	})
	if err := h(context.Background()); err == nil {
		t.Fatal("want error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (non-retryable)", attempts)
	}
}

func TestBreakerDropsUnderFailure(t *testing.T) {
	b := resilience.Breaker(
		resilience.WithWindow(10*time.Second),
		resilience.WithProbeInterval(time.Hour),
	)
	fail := func(ctx context.Context) error { return errors.New("boom") }
	h := b(fail)

	dropped := 0
	for i := 0; i < 200; i++ {
		if errors.Is(h(context.Background()), resilience.ErrCircuitOpen) {
			dropped++
		}
	}
	if dropped == 0 {
		t.Fatal("expected the breaker to drop requests under sustained failure")
	}
}

func TestBreakerAcceptableDoesNotTrip(t *testing.T) {
	acceptable := func(err error) bool { return err == nil || err.Error() == "client-error" }
	b := resilience.Breaker(
		resilience.WithWindow(time.Second),
		resilience.WithProbeInterval(time.Hour),
		resilience.WithAcceptable(acceptable),
	)
	clientErr := func(ctx context.Context) error { return errors.New("client-error") }
	h := b(clientErr)

	for i := 0; i < 200; i++ {
		if errors.Is(h(context.Background()), resilience.ErrCircuitOpen) {
			t.Fatalf("acceptable errors must not trip the breaker (dropped at %d)", i)
		}
	}
}

func TestRetryPerAttemptTimeout(t *testing.T) {
	// Each attempt exceeds the per-attempt timeout, so all attempts should time out.
	h := resilience.Retry(
		resilience.WithMaxAttempts(2),
		resilience.WithBackoff(time.Millisecond, time.Millisecond),
		resilience.WithPerAttemptTimeout(10*time.Millisecond),
	)(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			return nil
		}
	})

	start := time.Now()
	err := h(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("per-attempt timeout did not bound the attempt: took %v", elapsed)
	}
}

func TestHedgeReturnsPrimaryWhenFast(t *testing.T) {
	h := resilience.Hedge(time.Second)(func(ctx context.Context) error { return nil })
	if err := h(context.Background()); err != nil {
		t.Fatalf("Hedge() = %v, want nil", err)
	}
}

func TestHedgeReturnsBackupWhenPrimarySlow(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := resilience.Hedge(5 * time.Millisecond)(func(ctx context.Context) error {
		if calls.Add(1) == 1 {
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	})

	if err := h(ctx); err != nil {
		t.Fatalf("Hedge() = %v, want nil (backup should succeed)", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2 (primary + backup)", calls.Load())
	}
}

func TestRetryMaxAttemptsNonPositiveRunsOnce(t *testing.T) {
	for _, n := range []int{0, -1} {
		attempts := 0
		h := resilience.Retry(resilience.WithMaxAttempts(n))(
			func(ctx context.Context) error {
				attempts++
				return errors.New("always fail")
			})
		if err := h(context.Background()); err == nil {
			t.Fatalf("maxAttempts=%d: want error, got nil", n)
		}
		if attempts != 1 {
			t.Fatalf("maxAttempts=%d: attempts = %d, want 1", n, attempts)
		}
	}
}

func TestHedgeRecoversFromPanic(t *testing.T) {
	h := resilience.Hedge(time.Second)(func(ctx context.Context) error {
		panic("boom")
	})
	if err := h(context.Background()); err == nil {
		t.Fatal("Hedge() = nil, want an error surfaced from the panic")
	}
}

func TestHedgeCancelsLoser(t *testing.T) {
	var calls atomic.Int32
	primaryCanceled := make(chan struct{})
	h := resilience.Hedge(5 * time.Millisecond)(func(ctx context.Context) error {
		if calls.Add(1) == 1 {
			<-ctx.Done()
			close(primaryCanceled)
			return ctx.Err()
		}
		return nil
	})

	if err := h(context.Background()); err != nil {
		t.Fatalf("Hedge() = %v, want nil", err)
	}
	select {
	case <-primaryCanceled:
	case <-time.After(time.Second):
		t.Fatal("primary request was not canceled after the backup won")
	}
}

func TestBulkheadFailsFastWhenFull(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	h := resilience.Bulkhead(1)(func(ctx context.Context) error {
		close(entered)
		<-release
		return nil
	})

	go func() { _ = h(context.Background()) }()
	<-entered // the single slot is now held by the goroutine

	err := h(context.Background())
	if !errors.Is(err, resilience.ErrBulkheadFull) {
		t.Fatalf("want ErrBulkheadFull, got %v", err)
	}
	close(release)
}

func TestBulkheadAllowsWithinCapacity(t *testing.T) {
	h := resilience.Bulkhead(2)(func(ctx context.Context) error { return nil })
	if err := h(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBulkheadDisabledWhenNonPositive(t *testing.T) {
	h := resilience.Bulkhead(0)(func(ctx context.Context) error { return nil })
	if err := h(context.Background()); err != nil {
		t.Fatalf("disabled bulkhead must pass through: %v", err)
	}
}

func TestDeadlineReturnsTypedTimeout(t *testing.T) {
	h := resilience.Deadline(10 * time.Millisecond)(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	err := h(context.Background())
	if !errors.Is(err, errno.ErrTimeout) {
		t.Fatalf("want errno.ErrTimeout, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded (via wrap), got %v", err)
	}
}

func TestDeadlinePropagatesParentBudget(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	h := resilience.Deadline(time.Hour)(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	start := time.Now()
	err := h(parent)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("parent deadline was not propagated: took %v", elapsed)
	}
}
