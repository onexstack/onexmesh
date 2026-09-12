// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package app

// LifecycleStage identifies a point in the application lifecycle.
type LifecycleStage int

const (
	// StageBeforeStart runs before the RunFunc.
	StageBeforeStart LifecycleStage = iota
	// StageAfterStart runs after the RunFunc's servers have started (the
	// registration point).
	StageAfterStart
	// StageBeforeStop runs before shutdown begins (the deregistration point).
	StageBeforeStop
	// StageAfterStop runs after the RunFunc has returned.
	StageAfterStop
)

// WithLifecycleHook registers a hook at the given lifecycle stage. This
// supersedes the legacy WithPreRunHook / WithPreShutdownHook options while
// remaining compatible with them.
func WithLifecycleHook(stage LifecycleStage, hook LifecycleHook) Option {
	return func(a *App) {
		switch stage {
		case StageBeforeStart:
			a.preRunHooks = append(a.preRunHooks, hook)
		case StageAfterStart:
			a.afterStartHooks = append(a.afterStartHooks, hook)
		case StageBeforeStop:
			a.beforeStopHooks = append(a.beforeStopHooks, hook)
		case StageAfterStop:
			a.preShutdownHooks = append(a.preShutdownHooks, hook)
		}
	}
}
