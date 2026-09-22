// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package registry

import (
	"testing"
	"time"
)

// TestDecodeOptionsMatchesHyphenatedKeys covers the matcher, which is the part
// of file-configuration that fails silently when it is wrong: a key that does
// not match its field leaves the default in place, the service starts, and
// nothing reports that the setting was ignored.
func TestDecodeOptionsMatchesHyphenatedKeys(t *testing.T) {
	type opts struct {
		DialTimeout time.Duration
		PortName    string
		Weight      float64
	}

	raw := map[string]any{
		// The spelling a config file uses: the flag name, which is kebab-case.
		"dial-timeout": "10s",
		"port-name":    "grpc",
		"weight":       100,
	}

	var out opts
	if err := DecodeOptions(raw, &out); err != nil {
		t.Fatalf("DecodeOptions() error = %v", err)
	}

	if out.DialTimeout != 10*time.Second {
		t.Errorf("DialTimeout = %v, want 10s (from \"dial-timeout\")", out.DialTimeout)
	}
	if out.PortName != "grpc" {
		t.Errorf("PortName = %q, want %q (from \"port-name\")", out.PortName, "grpc")
	}
	if out.Weight != 100 {
		t.Errorf("Weight = %v, want 100", out.Weight)
	}
}

// TestDecodeOptionsWeaklyTyped covers the values a YAML file writes loosely but
// Go holds strictly, so config behaves the same here as it does elsewhere in
// the application.
func TestDecodeOptionsWeaklyTyped(t *testing.T) {
	type opts struct {
		TTL      int
		Insecure bool
	}

	var out opts
	if err := DecodeOptions(map[string]any{"ttl": "5", "insecure": "true"}, &out); err != nil {
		t.Fatalf("DecodeOptions() error = %v", err)
	}
	if out.TTL != 5 {
		t.Errorf("TTL = %d, want 5 (from the string \"5\")", out.TTL)
	}
	if !out.Insecure {
		t.Error("Insecure = false, want true (from the string \"true\")")
	}
}
