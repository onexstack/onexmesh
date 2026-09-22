// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"testing"

	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// recordingBackend captures what Decode was handed, so a test can observe the
// config-file -> backend path without standing up a real registry client. It
// stands in for polaris/etcd/... whose Options are equally unreachable from
// this package — which is the whole reason Complete exists.
type recordingBackend struct {
	name string
	raw  map[string]any
}

var _ registry.Backend = (*recordingBackend)(nil)

// lastRecording is the backend instance AddFlags most recently constructed.
var lastRecording *recordingBackend

func (b *recordingBackend) Name() string { return b.name }

func (b *recordingBackend) AddFlags(fs *pflag.FlagSet, prefix string) {
	// The bound variables are deliberately discarded: this backend exists to
	// record what arrives by file, and the flag values are asserted through
	// Changed rather than read back.
	fs.StringVar(new(string), prefix+".addr", "", "Test address.")
	fs.IntVar(new(int), prefix+".ttl", 5, "Test TTL.")
}

func (b *recordingBackend) Decode(raw map[string]any) error {
	b.raw = raw
	return nil
}

func (b *recordingBackend) NewRegistrar(string, int, string) (registry.Registrar, error) {
	return nil, nil
}

func (b *recordingBackend) NewDiscovery() (registry.Discovery, error) { return nil, nil }

func init() {
	registry.RegisterBackend("testrecording", func() registry.Backend {
		lastRecording = &recordingBackend{name: "testrecording"}
		return lastRecording
	})
}

// newRecordingOptions returns options wired to the recording backend, with its
// flags bound exactly as the app binds them.
func newRecordingOptions(t *testing.T, args ...string) *RegistryOptions {
	t.Helper()

	o := NewRegistryOptions()
	o.Type = "testrecording"

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	o.AddFlags(fs, "registry")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return o
}

// TestRegistryOptionsCompleteAppliesConfigFile is the regression test for the
// bug this mechanism was added to fix: a backend setting written into a config
// file used to reach nothing at all.
func TestRegistryOptionsCompleteAppliesConfigFile(t *testing.T) {
	o := newRecordingOptions(t)

	// The shape viper's ",remain" produces for a file that carries
	// registry.testrecording.{addr,ttl}.
	o.Options = map[string]any{
		"testrecording": map[string]any{"addr": "polaris.infra-devops.svc.cluster.local:80", "ttl": 7},
	}

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if got := lastRecording.raw["addr"]; got != "polaris.infra-devops.svc.cluster.local:80" {
		t.Errorf("backend received addr = %v, want the value from the config file", got)
	}
	if got := lastRecording.raw["ttl"]; got != 7 {
		t.Errorf("backend received ttl = %v, want 7", got)
	}
}

// TestRegistryOptionsCompleteFlagBeatsConfig pins the precedence order: an
// explicit flag is not overridden by the file. The flag binding has already
// written the explicit value into the backend, so applying the file on top of
// it would silently reverse the two.
func TestRegistryOptionsCompleteFlagBeatsConfig(t *testing.T) {
	o := newRecordingOptions(t, "--registry.testrecording.addr=from-flag")

	o.Options = map[string]any{
		"testrecording": map[string]any{"addr": "from-file", "ttl": 7},
	}

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if _, ok := lastRecording.raw["addr"]; ok {
		t.Error("the config file overrode an explicitly set flag")
	}
	// The settings that were not flagged still come from the file.
	if got := lastRecording.raw["ttl"]; got != 7 {
		t.Errorf("backend received ttl = %v, want 7", got)
	}
}

// TestRegistryOptionsValidateRejectsUnknownBackend pins that a misspelled
// backend name is reported rather than dropped, which is the failure mode the
// ",remain" option would otherwise introduce.
func TestRegistryOptionsValidateRejectsUnknownBackend(t *testing.T) {
	o := NewRegistryOptions()
	o.Options = map[string]any{"polariss": map[string]any{"addr": "typo"}}

	errs := o.Validate()
	if len(errs) == 0 {
		t.Fatal("Validate() accepted an unregistered backend name")
	}
}

// TestRegistryOptionsCompleteWithoutAddFlags covers programmatic construction,
// where there is no flag set to consult.
func TestRegistryOptionsCompleteWithoutAddFlags(t *testing.T) {
	o := NewRegistryOptions()
	o.Type = "testrecording"
	o.Options = map[string]any{"testrecording": map[string]any{"addr": "10.0.0.1:8091"}}

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got := lastRecording.raw["addr"]; got != "10.0.0.1:8091" {
		t.Errorf("backend received addr = %v, want 10.0.0.1:8091", got)
	}
}
