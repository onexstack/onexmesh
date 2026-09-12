// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// fakeClock is a controllable clock for exercising TTL/throttle logic.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Unix(1000, 0)} }

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	f.t = f.t.Add(d)
	f.mu.Unlock()
}

// mockDiscovery records how many times GetService is invoked.
type mockDiscovery struct {
	calls   atomic.Int32
	handler func(ctx context.Context, name string) ([]*registry.ServiceInstance, error)
}

func (m *mockDiscovery) GetService(ctx context.Context, name string) ([]*registry.ServiceInstance, error) {
	m.calls.Add(1)
	return m.handler(ctx, name)
}

func (m *mockDiscovery) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	return nil, errors.New("not implemented")
}

func sampleInstances() []*registry.ServiceInstance {
	return []*registry.ServiceInstance{{ID: "n1", Name: "svc", Endpoints: []string{"grpc://127.0.0.1:9090"}}}
}

func TestCacheDeduplicatesSequential(t *testing.T) {
	fc := newFakeClock()

	m := &mockDiscovery{handler: func(context.Context, string) ([]*registry.ServiceInstance, error) {
		return sampleInstances(), nil
	}}
	c := New(m, WithNow(fc.Now), WithTTL(time.Minute), WithNodeTTL(30*time.Second))

	for i := 0; i < 3; i++ {
		if _, err := c.GetService(context.Background(), "svc"); err != nil {
			t.Fatalf("GetService() error = %v", err)
		}
	}
	if got := m.calls.Load(); got != 1 {
		t.Fatalf("underlying calls = %d, want 1", got)
	}
}

func TestCacheDeduplicatesConcurrent(t *testing.T) {
	fc := newFakeClock()

	m := &mockDiscovery{handler: func(context.Context, string) ([]*registry.ServiceInstance, error) {
		time.Sleep(5 * time.Millisecond)
		return sampleInstances(), nil
	}}
	c := New(m, WithNow(fc.Now))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.GetService(context.Background(), "svc"); err != nil {
				t.Errorf("GetService() error = %v", err)
			}
		}()
	}
	wg.Wait()
	if got := m.calls.Load(); got != 1 {
		t.Fatalf("underlying calls = %d, want 1", got)
	}
}

func TestCacheExpiresAfterTTL(t *testing.T) {
	fc := newFakeClock()

	m := &mockDiscovery{handler: func(context.Context, string) ([]*registry.ServiceInstance, error) {
		return sampleInstances(), nil
	}}
	c := New(m, WithNow(fc.Now), WithTTL(time.Second), WithNodeTTL(time.Second), WithMinRetryInterval(0))

	if _, err := c.GetService(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	fc.Advance(2 * time.Second)
	if _, err := c.GetService(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	if got := m.calls.Load(); got != 2 {
		t.Fatalf("underlying calls = %d, want 2", got)
	}
}

func TestCacheDegradesOnError(t *testing.T) {
	fc := newFakeClock()

	var fail atomic.Bool
	m := &mockDiscovery{handler: func(context.Context, string) ([]*registry.ServiceInstance, error) {
		if fail.Load() {
			return nil, errors.New("registry down")
		}
		return sampleInstances(), nil
	}}
	c := New(m, WithNow(fc.Now), WithTTL(time.Second), WithNodeTTL(time.Second), WithMinRetryInterval(0))

	if _, err := c.GetService(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	fc.Advance(2 * time.Second)

	insts, err := c.GetService(context.Background(), "svc")
	if err != nil {
		t.Fatalf("expected stale fallback, got error %v", err)
	}
	if len(insts) != 1 {
		t.Fatalf("len(instances) = %d, want 1", len(insts))
	}
}

func TestCacheThrottlesRefresh(t *testing.T) {
	fc := newFakeClock()

	m := &mockDiscovery{handler: func(context.Context, string) ([]*registry.ServiceInstance, error) {
		return sampleInstances(), nil
	}}
	c := New(m, WithNow(fc.Now), WithTTL(time.Second), WithNodeTTL(time.Second), WithMinRetryInterval(5*time.Second))

	if _, err := c.GetService(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	// Cache expired but within the retry interval: serve stale, no refresh.
	fc.Advance(2 * time.Second)
	if _, err := c.GetService(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	if got := m.calls.Load(); got != 1 {
		t.Fatalf("underlying calls = %d, want 1 (throttled)", got)
	}
}
