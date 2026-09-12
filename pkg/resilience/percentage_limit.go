// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import (
	"time"

	"github.com/onexstack/onexmesh/pkg/core/collection"
)

// retryPercentageLimiter bounds the fraction of requests that are retries,
// protecting a downstream service from retry storms. It tracks total and retry
// counts over a sliding window and refuses a retry once the retry ratio reaches
// the configured limit.
type retryPercentageLimiter struct {
	ratio float64
	total *collection.RollingWindow[int64, *collection.Bucket[int64]]
	retry *collection.RollingWindow[int64, *collection.Bucket[int64]]
}

const (
	percentageWindow  = 10 * time.Second
	percentageBuckets = 10
)

func newRetryPercentageLimiter(ratio float64) *retryPercentageLimiter {
	newBucket := func() *collection.Bucket[int64] { return &collection.Bucket[int64]{} }
	return &retryPercentageLimiter{
		ratio: ratio,
		total: collection.NewRollingWindow(newBucket, percentageBuckets, percentageWindow),
		retry: collection.NewRollingWindow(newBucket, percentageBuckets, percentageWindow),
	}
}

// observeTotal records an attempt (including the initial call).
func (p *retryPercentageLimiter) observeTotal() {
	p.total.Add(1)
}

// allowRetry reports whether a retry may be issued, recording the retry attempt
// if permitted. A ratio <= 0 disables the limit.
func (p *retryPercentageLimiter) allowRetry() bool {
	if p.ratio <= 0 {
		return true
	}

	var total, retry int64
	p.total.Reduce(func(b *collection.Bucket[int64]) { total += b.Sum })
	p.retry.Reduce(func(b *collection.Bucket[int64]) { retry += b.Sum })

	if total == 0 {
		return true
	}
	if float64(retry)/float64(total) >= p.ratio {
		return false
	}
	p.retry.Add(1)
	return true
}
