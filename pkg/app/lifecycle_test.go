// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"testing"
)

func TestWithLifecycleHookStages(t *testing.T) {
	var beforeStart, afterStart, beforeStop, afterStop int

	a := NewApp("test", "test",
		WithLifecycleHook(StageBeforeStart, func(context.Context) error { beforeStart++; return nil }),
		WithLifecycleHook(StageAfterStart, func(context.Context) error { afterStart++; return nil }),
		WithLifecycleHook(StageBeforeStop, func(context.Context) error { beforeStop++; return nil }),
		WithLifecycleHook(StageAfterStop, func(context.Context) error { afterStop++; return nil }),
	)

	if len(a.preRunHooks) != 1 {
		t.Fatalf("preRunHooks = %d, want 1", len(a.preRunHooks))
	}
	if len(a.afterStartHooks) != 1 {
		t.Fatalf("afterStartHooks = %d, want 1", len(a.afterStartHooks))
	}
	if len(a.beforeStopHooks) != 1 {
		t.Fatalf("beforeStopHooks = %d, want 1", len(a.beforeStopHooks))
	}
	if len(a.preShutdownHooks) != 1 {
		t.Fatalf("preShutdownHooks = %d, want 1", len(a.preShutdownHooks))
	}
}
