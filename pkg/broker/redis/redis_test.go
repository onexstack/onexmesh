// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package redis

import "testing"

func TestDecodeMessage(t *testing.T) {
	values := map[string]any{
		"header": `{"a":"1"}`,
		"body":   "hello",
	}
	m := decodeMessage(values)

	if v, ok := m.Header.Get("a"); !ok || v != "1" {
		t.Fatalf("header[a] = %q, %v, want 1, true", v, ok)
	}
	if string(m.Body) != "hello" {
		t.Fatalf("body = %q, want hello", m.Body)
	}
}

func TestDecodeMessageEmpty(t *testing.T) {
	m := decodeMessage(map[string]any{})
	if m == nil || m.Header == nil {
		t.Fatal("decodeMessage should return a non-nil message with a header")
	}
}
