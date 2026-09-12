// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package limit

import (
	"errors"
	"testing"
)

func TestLimitTryBorrow(t *testing.T) {
	l := NewLimit(2)

	if !l.TryBorrow() {
		t.Fatal("first borrow should succeed")
	}
	if !l.TryBorrow() {
		t.Fatal("second borrow should succeed")
	}
	if l.TryBorrow() {
		t.Fatal("third borrow should fail at capacity 2")
	}

	if err := l.Return(); err != nil {
		t.Fatalf("Return() error: %v", err)
	}
	if !l.TryBorrow() {
		t.Fatal("borrow should succeed after a return")
	}
}

func TestLimitReturnMoreThanBorrowed(t *testing.T) {
	l := NewLimit(1)

	if err := l.Return(); !errors.Is(err, ErrLimitReturn) {
		t.Fatalf("Return() = %v, want ErrLimitReturn", err)
	}
}
