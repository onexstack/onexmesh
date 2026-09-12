// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config_test

import (
	"testing"

	"github.com/onexstack/onexmesh/pkg/config"
	_ "github.com/onexstack/onexmesh/pkg/config/source/file"
	_ "github.com/onexstack/onexmesh/pkg/config/source/polaris"
)

func TestSourcesRegistered(t *testing.T) {
	for _, name := range []string{"file", "polaris"} {
		if !config.RegisteredSource(name) {
			t.Fatalf("source %q not registered", name)
		}
	}
}

func TestCreateUnknownSource(t *testing.T) {
	if _, err := config.CreateSource("unknown", nil); err == nil {
		t.Fatal("expected error for unknown source")
	}
}
