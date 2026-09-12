// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package metadata

import (
	"context"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	ctx := NewContext(context.Background(), Metadata{"a": "1", "b": "2"})

	md, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext() not ok")
	}
	if got, _ := md.Get("a"); got != "1" {
		t.Fatalf("md[a] = %q, want %q", got, "1")
	}
}

func TestMergeContextOverwrite(t *testing.T) {
	ctx := NewContext(context.Background(), Metadata{"a": "old", "keep": "k"})

	// Overwrite false: existing "a" wins.
	ctx = MergeContext(ctx, Metadata{"a": "new"}, false)
	md, _ := FromContext(ctx)
	if got, _ := md.Get("a"); got != "old" {
		t.Fatalf("non-overwrite md[a] = %q, want %q", got, "old")
	}

	// Overwrite true: patch wins.
	ctx = MergeContext(ctx, Metadata{"a": "new"}, true)
	md, _ = FromContext(ctx)
	if got, _ := md.Get("a"); got != "new" {
		t.Fatalf("overwrite md[a] = %q, want %q", got, "new")
	}
}

func TestGetWithoutMetadata(t *testing.T) {
	if _, ok := Get(context.Background(), "x"); ok {
		t.Fatal("Get() on a bare context should not be ok")
	}
}

func TestSetAndDelete(t *testing.T) {
	ctx := Set(context.Background(), "k", "v")
	if got, ok := Get(ctx, "k"); !ok || got != "v" {
		t.Fatalf("Get(k) = %q, %v, want v, true", got, ok)
	}

	ctx = Delete(ctx, "k")
	if _, ok := Get(ctx, "k"); ok {
		t.Fatal("Get(k) after Delete should not be ok")
	}
}
