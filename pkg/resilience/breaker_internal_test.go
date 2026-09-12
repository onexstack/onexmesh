// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import "testing"

// TestHistoryCountsWorkingAndFailingIndependently guards the fix for the
// history() regression: a single bucket containing both successes and failures
// must count toward both workingBuckets and failingBuckets, not reset one to
// zero when the other is seen.
func TestHistoryCountsWorkingAndFailingIndependently(t *testing.T) {
	cfg := defaultBreakerConfig()
	cfg.buckets = 4
	b := newSREBreaker(cfg)

	b.markSuccess()
	b.markFailure()

	r := b.history()
	if r.workingBuckets != 1 {
		t.Fatalf("workingBuckets = %d, want 1", r.workingBuckets)
	}
	if r.failingBuckets != 1 {
		t.Fatalf("failingBuckets = %d, want 1", r.failingBuckets)
	}
}
