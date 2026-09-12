// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package stat exposes smoothed system-level statistics. The CPU usage is
// sampled in the background and exponentially smoothed so resilience primitives
// (e.g. the adaptive load shedder) can make overload decisions.
package stat

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	// cpuRefreshInterval and beta together approximate the average CPU load over
	// the past ~5 seconds.
	cpuRefreshInterval = 250 * time.Millisecond
	beta               = 0.95
)

var (
	cpuUsage  atomic.Int64
	startOnce sync.Once
)

// CpuUsage returns the current smoothed CPU usage in per-mille (0-1000). It
// starts a background sampler on first use.
func CpuUsage() int64 {
	startOnce.Do(startSampler)
	return cpuUsage.Load()
}

func startSampler() {
	go func() {
		ticker := time.NewTicker(cpuRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			cur := refreshCPU()
			prev := cpuUsage.Load()
			// cpu = cpuPrev*beta + cpuCur*(1-beta)
			cpuUsage.Store(int64(float64(prev)*beta + float64(cur)*(1-beta)))
		}
	}()
}
