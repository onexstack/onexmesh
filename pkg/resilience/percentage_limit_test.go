// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import "testing"

func TestRetryPercentageLimiter(t *testing.T) {
	l := newRetryPercentageLimiter(0.5) // at most 50% retries

	// First request: 1 total, 0 retries -> retry allowed.
	l.observeTotal()
	if !l.allowRetry() {
		t.Fatal("first retry should be allowed (0/1 retry ratio)")
	}

	// Second request: 2 total, 1 retry -> ratio 0.5, retry denied (>= limit).
	l.observeTotal()
	if l.allowRetry() {
		t.Fatal("retry should be denied at 50% retry ratio")
	}
}

func TestRetryPercentageLimiterNoLimit(t *testing.T) {
	l := newRetryPercentageLimiter(0) // ratio 0 means unlimited

	l.observeTotal()
	if !l.allowRetry() {
		t.Fatal("retry should always be allowed when ratio is 0")
	}
}
