// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package errno

import (
	"errors"
	"net/http"
	"testing"

	"github.com/onexstack/onexstack/pkg/errorsx"
)

func TestClassifiersByCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		fn   func(error) bool
		want bool
	}{
		{"not_found", errorsx.New(http.StatusNotFound, "NotFound", "nf"), IsNotFound, true},
		{"bad_request", errorsx.New(http.StatusBadRequest, "BadRequest", "br"), IsBadRequest, true},
		{"internal", errorsx.New(http.StatusInternalServerError, "Internal", "e"), IsInternal, true},
		{"unavailable", errorsx.New(http.StatusServiceUnavailable, "Unavailable", "u"), IsServiceUnavailable, true},
		{"not_found_mismatch", errorsx.New(http.StatusForbidden, "PermissionDenied", "pd"), IsNotFound, false},
	}
	for _, tt := range tests {
		if got := tt.fn(tt.err); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestClassifiersByReason(t *testing.T) {
	if !IsCircuitOpen(ErrCircuitOpen) {
		t.Fatal("IsCircuitOpen(ErrCircuitOpen) = false, want true")
	}
	if !IsTimeout(ErrTimeout) {
		t.Fatal("IsTimeout(ErrTimeout) = false, want true")
	}
	if !IsRateLimited(ErrRateLimited) {
		t.Fatal("IsRateLimited(ErrRateLimited) = false, want true")
	}
	if !IsOverloaded(ErrServiceOverloaded) {
		t.Fatal("IsOverloaded(ErrServiceOverloaded) = false, want true")
	}
}

func TestClassifierUnwraps(t *testing.T) {
	wrapped := errorsx.New(http.StatusNotFound, "NotFound", "nf").Wrap(errors.New("cause"))
	if !IsNotFound(wrapped) {
		t.Fatal("IsNotFound should unwrap the error chain")
	}
}
