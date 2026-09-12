// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package contextx provides context helpers. ValueOnlyFrom lets an asynchronous
// task inherit a parent's values (trace id, metadata, ...) without inheriting
// its cancellation or deadline, so a finished request does not abort the task.
package contextx

import (
	"context"
	"time"
)

type valueOnlyContext struct {
	context.Context
}

func (valueOnlyContext) Deadline() (time.Time, bool) { return time.Time{}, false }

func (valueOnlyContext) Done() <-chan struct{} { return nil }

func (valueOnlyContext) Err() error { return nil }

// ValueOnlyFrom returns a context carrying only ctx's values, dropping its
// deadline, cancellation and error propagation. It delegates to the standard
// library's context.WithoutCancel (Go 1.21+), which has identical semantics.
func ValueOnlyFrom(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}
