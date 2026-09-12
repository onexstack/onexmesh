// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package contextx

import (
	"context"
	"testing"
	"time"
)

func TestValueOnlyFrom(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	ctx := ValueOnlyFrom(parent)
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("Deadline() reported ok=true")
	}
	if ctx.Done() != nil {
		t.Fatal("Done() != nil")
	}
	if ctx.Err() != nil {
		t.Fatalf("Err() = %v, want nil", ctx.Err())
	}

	// Values still propagate.
	withVal := context.WithValue(parent, "key", "value")
	ctx = ValueOnlyFrom(withVal)
	if v := ctx.Value("key"); v != "value" {
		t.Fatalf("Value(key) = %v, want value", v)
	}
}
