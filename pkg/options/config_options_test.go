// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/onexstack/onexmesh/pkg/config"
)

// recordingConfigBackend captures what Decode was handed, so a test can observe
// the config-file -> backend path without standing up a config center.
type recordingConfigBackend struct {
	name string
	raw  map[string]any
}

var _ config.Backend = (*recordingConfigBackend)(nil)

var lastConfigRecording *recordingConfigBackend

func (b *recordingConfigBackend) Name() string { return b.name }

func (b *recordingConfigBackend) AddFlags(fs *pflag.FlagSet, prefix string) {
	// Bound variables are discarded: this backend exists to record what arrives
	// by file, and the flag values are asserted through Changed, not read back.
	fs.StringVar(new(string), prefix+".addr", "", "Test address.")
	fs.IntVar(new(int), prefix+".ttl", 5, "Test TTL.")
}

func (b *recordingConfigBackend) Decode(raw map[string]any) error {
	b.raw = raw
	return nil
}

func (b *recordingConfigBackend) NewSource() (config.Source, error) { return nil, nil }

func init() {
	config.RegisterBackend("testrecordingcfg", func() config.Backend {
		lastConfigRecording = &recordingConfigBackend{name: "testrecordingcfg"}
		return lastConfigRecording
	})
}

func newConfigRecordingOptions(t *testing.T, args ...string) *ConfigOptions {
	t.Helper()

	o := NewConfigOptions()
	o.Type = "testrecordingcfg"

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	o.AddFlags(fs, "config-source")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return o
}

// TestConfigSourceSectionIsReadableAlongsideTheConfigFlag is the regression test
// for the reason this section is called `config-source` and not `config`.
//
// The application binds its own --config flag into the same viper instance, and
// viper resolves a nested key whose parent also names a bound flag to nil
// (isPathShadowedInFlatMap) — before it consults the environment or the config
// file. So a `config:` section would read as empty even though it was present
// and correct, and the failure would look like a section nobody filled in.
func TestConfigSourceSectionIsReadableAlongsideTheConfigFlag(t *testing.T) {
	const yamlDoc = `
config-source:
  type: file
  file:
    paths: [/etc/remote.yaml]
`
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(yamlDoc)); err != nil {
		t.Fatal(err)
	}

	opts := NewServerOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	// The application's own config-file flag, bound the same way the framework
	// binds it. This is what would shadow a `config` section.
	var cfgFile string
	fs.StringVarP(&cfgFile, "config", "c", "", "config file")
	opts.AddFlags(fs)

	if err := v.BindPFlags(fs); err != nil {
		t.Fatal(err)
	}
	if err := v.Unmarshal(opts); err != nil {
		t.Fatal(err)
	}
	if err := opts.Complete(); err != nil {
		t.Fatal(err)
	}

	if got := opts.Config.Type; got != "file" {
		t.Fatalf("config-source.type = %q, want file — the section was shadowed or never reached", got)
	}
	if errs := opts.Config.Validate(); len(errs) > 0 {
		t.Fatalf("Validate() rejected a valid section: %v", errs)
	}
}

func TestConfigSourceCompleteAppliesConfigFile(t *testing.T) {
	o := newConfigRecordingOptions(t)

	// The shape viper's ",remain" produces for config-source.testrecordingcfg.*
	o.Options = map[string]any{
		"testrecordingcfg": map[string]any{"addr": "polaris.infra-devops.svc.cluster.local:8093", "ttl": 7},
	}

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if got := lastConfigRecording.raw["addr"]; got != "polaris.infra-devops.svc.cluster.local:8093" {
		t.Errorf("backend received addr = %v, want the value from the config file", got)
	}
	if got := lastConfigRecording.raw["ttl"]; got != 7 {
		t.Errorf("backend received ttl = %v, want 7", got)
	}
}

func TestConfigSourceCompleteFlagBeatsConfig(t *testing.T) {
	o := newConfigRecordingOptions(t, "--config-source.testrecordingcfg.addr=from-flag")

	o.Options = map[string]any{
		"testrecordingcfg": map[string]any{"addr": "from-file", "ttl": 7},
	}

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if _, ok := lastConfigRecording.raw["addr"]; ok {
		t.Error("the config file overrode an explicitly set flag")
	}
	if got := lastConfigRecording.raw["ttl"]; got != 7 {
		t.Errorf("backend received ttl = %v, want 7", got)
	}
}

// TestConfigSourceValidateRejectsUnknownType pins that a misspelled type is
// reported. Fail-open is for a source that is unreachable, not for a name that
// does not exist: a typo accepted silently would disable the mechanism the
// section exists to configure, and nothing would say so.
func TestConfigSourceValidateRejectsUnknownType(t *testing.T) {
	o := NewConfigOptions()
	o.Type = "polariss"

	errs := o.Validate()
	if len(errs) == 0 {
		t.Fatal("Validate() accepted an unregistered type")
	}
	if msg := errs[0].Error(); !strings.Contains(msg, "polaris") {
		t.Errorf("error %q should list the known types", msg)
	}
}

func TestConfigSourceValidateRejectsUnknownSection(t *testing.T) {
	o := NewConfigOptions()
	o.Options = map[string]any{"polariss": map[string]any{"addresses": "typo"}}

	if errs := o.Validate(); len(errs) == 0 {
		t.Fatal("Validate() accepted settings for an unregistered source")
	}
}

// TestConfigSourceDisabledLoadsNothing pins that the section is inert by default:
// no source, nothing loaded, no failure even in strict mode.
func TestConfigSourceDisabledLoadsNothing(t *testing.T) {
	o := NewConfigOptions()
	o.Strict = true

	if o.Enabled() {
		t.Fatal("a fresh ConfigOptions reports itself enabled")
	}
	tree, err := o.LoadConfigTree(context.Background())
	if err != nil {
		t.Fatalf("LoadConfigTree() error = %v, want none for a disabled source", err)
	}
	if tree != nil {
		t.Fatalf("LoadConfigTree() = %v, want nil", tree)
	}
	if err := o.WatchConfigTree(context.Background(), func(map[string]any) {}); err != nil {
		t.Fatalf("WatchConfigTree() error = %v, want none for a disabled source", err)
	}
}

// TestConfigSourceUnreachableIsNotFatalByDefault pins the default policy: a
// source that cannot be read warns and yields nothing, so one build runs both
// with and without a config center.
func TestConfigSourceUnreachableIsNotFatalByDefault(t *testing.T) {
	o := NewConfigOptions()
	o.Type = "file"
	o.Options = map[string]any{"file": map[string]any{"paths": []string{"/nonexistent/remote-config.yaml"}}}

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	tree, err := o.LoadConfigTree(context.Background())
	if err != nil {
		t.Fatalf("LoadConfigTree() error = %v, want the failure absorbed", err)
	}
	if tree != nil {
		t.Fatalf("LoadConfigTree() = %v, want nil", tree)
	}
}

// TestConfigSourceUnreachableIsFatalWhenStrict pins the other side of the
// policy: an operator who asked for the config center to be required gets a
// startup failure rather than a service quietly running on stale settings.
func TestConfigSourceUnreachableIsFatalWhenStrict(t *testing.T) {
	o := NewConfigOptions()
	o.Type = "file"
	o.Strict = true
	o.Options = map[string]any{"file": map[string]any{"paths": []string{"/nonexistent/remote-config.yaml"}}}

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if _, err := o.LoadConfigTree(context.Background()); err == nil {
		t.Fatal("LoadConfigTree() returned no error in strict mode")
	}
}

// TestConfigSourceFileLoadsATree exercises the whole path end to end without a
// config center: the section names a source, the source is built from the
// decoded settings, and the merged tree comes back.
func TestConfigSourceFileLoadsATree(t *testing.T) {
	dir := t.TempDir()
	remote := filepath.Join(dir, "remote.yaml")
	if err := os.WriteFile(remote, []byte("log:\n  level: debug\nport: 9090\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	o := NewConfigOptions()
	o.Type = "file"
	o.Options = map[string]any{"file": map[string]any{"paths": []string{remote}}}

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	tree, err := o.LoadConfigTree(context.Background())
	if err != nil {
		t.Fatalf("LoadConfigTree() error = %v", err)
	}
	if tree == nil {
		t.Fatal("LoadConfigTree() returned no tree for a readable file")
	}
	if got := tree["port"]; got != 9090 {
		t.Errorf("port = %v (%T), want 9090", got, got)
	}
	logSection, ok := tree["log"].(map[string]any)
	if !ok {
		t.Fatalf("log = %v, want a nested mapping", tree["log"])
	}
	if got := logSection["level"]; got != "debug" {
		t.Errorf("log.level = %v, want debug", got)
	}

	if err := o.CloseConfigSource(); err != nil {
		t.Fatalf("CloseConfigSource() error = %v", err)
	}
}
