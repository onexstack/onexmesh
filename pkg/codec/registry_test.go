// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import "testing"

func TestRegistryDefaults(t *testing.T) {
	for _, name := range []string{"json", "proto"} {
		m, err := Get(name)
		if err != nil {
			t.Fatalf("Get(%q) = %v", name, err)
		}
		if !Registered(name) {
			t.Fatalf("Registered(%q) = false", name)
		}
		if m == nil {
			t.Fatalf("Get(%q) returned nil marshaler", name)
		}
	}
}

func TestRegisterAndGet(t *testing.T) {
	const name = "custom"
	Register(name, JSON{})
	if !Registered(name) {
		t.Fatalf("Registered(%q) = false after Register", name)
	}
	if _, err := Get(name); err != nil {
		t.Fatalf("Get(%q) = %v", name, err)
	}
}

func TestGetUnknown(t *testing.T) {
	if _, err := Get("unknown"); err == nil {
		t.Fatal("expected error for unknown codec")
	}
	if Registered("unknown") {
		t.Fatal("Registered(unknown) = true")
	}
}
