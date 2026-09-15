// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package executors

import (
	"time"

	"github.com/onexstack/onexmesh/pkg/core/syncx"
)

// LessExecutor runs a task at most once per threshold interval.
type LessExecutor struct {
	threshold time.Duration
	lastTime  *syncx.AtomicDuration
}

// NewLessExecutor returns a LessExecutor with the given threshold interval.
func NewLessExecutor(threshold time.Duration) *LessExecutor {
	return &LessExecutor{threshold: threshold, lastTime: syncx.NewAtomicDuration()}
}

// DoOrDiscard runs fn if threshold has elapsed since the last execution, and
// discards it otherwise. It reports whether fn ran. The last-execution time is
// updated with a compare-and-swap so concurrent callers within a threshold
// window still execute at most once.
func (le *LessExecutor) DoOrDiscard(fn func()) bool {
	now := time.Duration(time.Now().UnixNano())
	for {
		lastTime := le.lastTime.Load()
		if lastTime != 0 && lastTime+le.threshold >= now {
			return false
		}
		if le.lastTime.CompareAndSwap(lastTime, now) {
			fn()
			return true
		}
	}
}
