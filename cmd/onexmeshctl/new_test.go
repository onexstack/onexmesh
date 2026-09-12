// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestFinalSegment(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"edu.course.student-api", "student-api"},
		{"student-api", "student-api"},
		{"", ""},
		{"edu.course", "course"},
	}
	for _, tt := range tests {
		if got := finalSegment(tt.in); got != tt.want {
			t.Errorf("finalSegment(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
