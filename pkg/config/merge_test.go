// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config

import "testing"

func TestDeepMerge(t *testing.T) {
	r := newReader()

	if err := r.Merge(&KeyValue{
		Key: "a", Value: []byte(`{"server":{"host":"127.0.0.1","port":8080},"name":"onex"}`), Format: "json",
	}); err != nil {
		t.Fatalf("merge a: %v", err)
	}
	if err := r.Merge(&KeyValue{
		Key: "b", Value: []byte(`{"server":{"port":9090},"debug":true}`), Format: "json",
	}); err != nil {
		t.Fatalf("merge b: %v", err)
	}

	// Later merge overrides server.port but preserves server.host.
	if got := r.Value("server.host").String(); got != "127.0.0.1" {
		t.Fatalf("server.host = %q, want 127.0.0.1", got)
	}
	if got := r.Value("server.port").Int(); got != 9090 {
		t.Fatalf("server.port = %d, want 9090", got)
	}
	if got := r.Value("name").String(); got != "onex" {
		t.Fatalf("name = %q, want onex", got)
	}
	if got := r.Value("debug").Bool(); got != true {
		t.Fatalf("debug = %v, want true", got)
	}
}

func TestDecodeToml(t *testing.T) {
	r := newReader()
	if err := r.Merge(&KeyValue{
		Key: "t", Value: []byte("name = \"onex\"\n[server]\nport = 8080\n"), Format: "toml",
	}); err != nil {
		t.Fatalf("merge toml: %v", err)
	}
	if got := r.Value("name").String(); got != "onex" {
		t.Fatalf("name = %q, want onex", got)
	}
	if got := r.Value("server.port").Int(); got != 8080 {
		t.Fatalf("server.port = %d, want 8080", got)
	}
}
