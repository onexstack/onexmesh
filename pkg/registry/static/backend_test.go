// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package static

import (
	"strings"
	"testing"
)

// TestDecodeMappingForm pins the one shape a config file may use for endpoints.
func TestDecodeMappingForm(t *testing.T) {
	b := &backend{opts: Options{Endpoints: map[string][]string{}}}

	err := b.Decode(map[string]any{
		"endpoints": map[string]any{
			"edu.onex.commerce-apiserver": []any{"http://127.0.0.1:8182", "http://127.0.0.1:8183"},
		},
	})
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	got := b.opts.Endpoints["edu.onex.commerce-apiserver"]
	if len(got) != 2 {
		t.Fatalf("endpoints = %v, want the two addresses", got)
	}
}

// TestDecodeRejectsTheFlagForm pins that copying the flag's shape into a file is
// an error and not a silent empty list, and that the error says which shape to
// write. The decoder's own message names the Go type ("map[string][]string"),
// which is not what someone editing YAML needs to read.
func TestDecodeRejectsTheFlagForm(t *testing.T) {
	b := &backend{opts: Options{Endpoints: map[string][]string{}}}

	err := b.Decode(map[string]any{
		"endpoints": []any{"edu.onex.commerce-apiserver=http://127.0.0.1:8182"},
	})
	if err == nil {
		t.Fatal("Decode() accepted the flag form, which leaves the address list empty")
	}
	// Without this the message is the decoder's, and the fix is not obvious.
	if !strings.Contains(err.Error(), "mapping of service name to addresses") {
		t.Errorf("error does not say what shape to write: %v", err)
	}
}
