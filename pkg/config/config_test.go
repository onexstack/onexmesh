// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/onexstack/onexmesh/pkg/config"
	"github.com/onexstack/onexmesh/pkg/config/source/file"
)

func TestConfigLoadAndValue(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	content := "server:\n  host: 127.0.0.1\n  port: 8080\n  enabled: true\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New(config.WithSource(file.New(p)))
	if err := cfg.Load(); err != nil {
		t.Fatal(err)
	}

	if got := cfg.Value("server.host").String(); got != "127.0.0.1" {
		t.Fatalf("server.host = %q, want 127.0.0.1", got)
	}
	if got := cfg.Value("server.port").Int(); got != 8080 {
		t.Fatalf("server.port = %d, want 8080", got)
	}
	if got := cfg.Value("server.enabled").Bool(); !got {
		t.Fatal("server.enabled = false, want true")
	}
}
