// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package polaris

import (
	"testing"

	"github.com/spf13/pflag"
)

// TestBackendDecodesHyphenatedFileKeys pins the matching that makes a
// configuration file usable here at all. Every multi-word setting is written
// with hyphens (file-group) and named without them (FileGroup); without the
// case- and hyphen-blind matcher they would decode to nothing and leave their
// defaults in place — a failure with no symptom, since the service starts and
// merely ignores what it was configured with.
func TestBackendDecodesHyphenatedFileKeys(t *testing.T) {
	b := &backend{}

	if err := b.Decode(map[string]any{
		"addresses":  []any{"10.0.0.1:8093"},
		"namespace":  "edu-onex",
		"file-group": "ox-iam-apiserver",
		"file-name":  "ox-iam-apiserver.yaml",
		"token":      "s3cret",
		"format":     "yaml",
	}); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if len(b.opts.Addresses) != 1 || b.opts.Addresses[0] != "10.0.0.1:8093" {
		t.Errorf("Addresses = %v, want [10.0.0.1:8093]", b.opts.Addresses)
	}
	if b.opts.FileGroup != "ox-iam-apiserver" {
		t.Errorf("FileGroup = %q, want ox-iam-apiserver", b.opts.FileGroup)
	}
	if b.opts.FileName != "ox-iam-apiserver.yaml" {
		t.Errorf("FileName = %q", b.opts.FileName)
	}
	if b.opts.Namespace != "edu-onex" {
		t.Errorf("Namespace = %q", b.opts.Namespace)
	}
	if b.opts.Token != "s3cret" {
		t.Errorf("Token = %q, want it decoded", b.opts.Token)
	}
}

// TestBackendFlagNamesMatchTheDocumentedKeys pins the flag spelling, because it
// is also the key spelling in a configuration file: a file addresses a backend
// under exactly its flag name, and a drift between the two would leave an
// operator with a setting that parses and does nothing.
func TestBackendFlagNamesMatchTheDocumentedKeys(t *testing.T) {
	b := &backend{}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	b.AddFlags(fs, "config-source.polaris")

	for _, name := range []string{
		"config-source.polaris.addresses",
		"config-source.polaris.namespace",
		"config-source.polaris.file-group",
		"config-source.polaris.file-name",
		"config-source.polaris.format",
		"config-source.polaris.token",
	} {
		if fs.Lookup(name) == nil {
			t.Errorf("flag %q is not registered", name)
		}
	}
}
