// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestShedderAllowsWhenNotOverloaded(t *testing.T) {
	old := systemOverloadChecker
	defer func() { systemOverloadChecker = old }()
	systemOverloadChecker = func(int64) bool { return false }

	s := newAdaptiveShedder()
	p, err := s.Allow()
	if err != nil {
		t.Fatalf("Allow() = %v, want nil when not overloaded", err)
	}
	p.Pass()
}

func TestShedderDropsWhenOverloaded(t *testing.T) {
	old := systemOverloadChecker
	defer func() { systemOverloadChecker = old }()
	systemOverloadChecker = func(int64) bool { return true }

	s := newAdaptiveShedder()
	// Simulate a large number of in-flight requests so highThru() is true.
	atomic.StoreInt64(&s.flying, 1000)
	s.avgFlyingLock.Lock()
	s.avgFlying = 1000
	s.avgFlyingLock.Unlock()

	if _, err := s.Allow(); !errors.Is(err, ErrServiceOverloaded) {
		t.Fatalf("Allow() = %v, want ErrServiceOverloaded", err)
	}
}
