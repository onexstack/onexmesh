// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import (
	"context"
	"errors"
	"time"

	"github.com/onexstack/onexmesh/pkg/errno"
)

// Deadline returns a middleware imposing the deadline (timeout) stability
// pattern. It bounds the handler by min(parent deadline, now+d): when the
// incoming context already carries an earlier deadline, that budget is
// propagated unchanged, so a downstream call never runs past the caller's
// deadline. When the deadline is the cause of failure it returns the typed
// errno.ErrTimeout (which still unwraps to context.DeadlineExceeded for
// errors.Is classification), giving both HTTP and gRPC paths a uniform timeout
// error.
func Deadline(d time.Duration) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			err := next(ctx)
			if errors.Is(err, context.DeadlineExceeded) {
				return errno.ErrTimeout.Wrap(err)
			}
			return err
		}
	}
}
