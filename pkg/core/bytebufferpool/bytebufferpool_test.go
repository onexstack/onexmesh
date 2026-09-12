// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bytebufferpool

import "testing"

func TestGetPutRoundTrip(t *testing.T) {
	b := Get()
	b.WriteString("hello")
	if b.String() != "hello" {
		t.Fatalf("String() = %q, want %q", b.String(), "hello")
	}
	Put(b)

	b2 := Get()
	if b2.Len() != 0 {
		t.Fatalf("reused buffer Len() = %d, want 0 (Reset on Put)", b2.Len())
	}
	Put(b2)
}

func TestIndex(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{0, 0},
		{64, 0},
		{128, 1},
		{1024, 4},
	}
	for _, tt := range tests {
		if got := index(tt.n); got != tt.want {
			t.Errorf("index(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}
