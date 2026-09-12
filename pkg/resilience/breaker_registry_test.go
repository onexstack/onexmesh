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

	"github.com/onexstack/onexmesh/pkg/resilience"
)

func TestBreakerRegistrySharesState(t *testing.T) {
	r := resilience.NewBreakerRegistry(
		resilience.WithWindow(10*time.Second),
		resilience.WithProbeInterval(time.Hour),
	)

	fail := func() error { return errors.New("boom") }
	dropped := 0
	for i := 0; i < 200; i++ {
		if errors.Is(r.Do("svc", fail, nil), resilience.ErrCircuitOpen) {
			dropped++
		}
	}
	if dropped == 0 {
		t.Fatal("expected the shared breaker to drop requests under sustained failure")
	}
}

func TestHedgeMultipleBackups(t *testing.T) {
	var calls atomic.Int32
	primaryCanceled := make(chan struct{})

	h := resilience.Hedge(5*time.Millisecond, resilience.WithBackupMaxRetries(2))(
		func(ctx context.Context) error {
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
	if calls.Load() < 2 {
		t.Fatalf("calls = %d, want >= 2 (primary + at least one backup)", calls.Load())
	}
}
