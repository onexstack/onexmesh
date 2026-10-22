// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package polaris

import (
	"errors"
	"testing"

	"github.com/polarismesh/polaris-go/api"
	polarisconfig "github.com/polarismesh/polaris-go/pkg/config"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// fakeConfigFileAPI records what the source asks for and returns canned content.
type fakeConfigFileAPI struct {
	lastRequest *model.GetConfigFileRequest

	content string
	err     error
}

var _ api.ConfigFileAPI = (*fakeConfigFileAPI)(nil)

func (f *fakeConfigFileAPI) GetConfigFile(namespace, fileGroup, fileName string) (model.ConfigFile, error) {
	return f.FetchConfigFile(&api.GetConfigFileRequest{
		GetConfigFileRequest: &model.GetConfigFileRequest{
			Namespace: namespace, FileGroup: fileGroup, FileName: fileName, Subscribe: true,
		},
	})
}

func (f *fakeConfigFileAPI) FetchConfigFile(req *api.GetConfigFileRequest) (model.ConfigFile, error) {
	f.lastRequest = req.GetConfigFileRequest
	if f.err != nil {
		return nil, f.err
	}
	return fakeConfigFile{content: f.content}, nil
}

func (f *fakeConfigFileAPI) CreateConfigFile(namespace, fileGroup, fileName, content string) error {
	return nil
}
func (f *fakeConfigFileAPI) UpdateConfigFile(namespace, fileGroup, fileName, content string) error {
	return nil
}
func (f *fakeConfigFileAPI) PublishConfigFile(namespace, fileGroup, fileName string) error {
	return nil
}
func (f *fakeConfigFileAPI) UpsertAndPublishConfigFile(namespace, fileGroup, fileName, content string) error {
	return nil
}
func (f *fakeConfigFileAPI) SDKContext() api.SDKContext { return nil }

// fakeConfigFile implements the handful of model.ConfigFile methods the source
// touches.
type fakeConfigFile struct {
	content string
}

var _ model.ConfigFile = fakeConfigFile{}

func (f fakeConfigFile) GetNamespace() string { return "" }
func (f fakeConfigFile) GetFileGroup() string { return "" }
func (f fakeConfigFile) GetFileName() string  { return "" }
func (f fakeConfigFile) GetFileMode() model.GetConfigFileRequestMode {
	return model.SDKMode
}
func (f fakeConfigFile) GetLabels() map[string]string               { return nil }
func (f fakeConfigFile) GetContent() string                         { return f.content }
func (f fakeConfigFile) HasContent() bool                           { return f.content != "" }
func (f fakeConfigFile) GetPersistent() model.Persistent            { return model.Persistent{} }
func (f fakeConfigFile) GetVersionName() string                     { return "" }
func (f fakeConfigFile) GetVersion() uint64                         { return 0 }
func (f fakeConfigFile) GetMd5() string                             { return "" }
func (f fakeConfigFile) AddChangeListener(model.OnConfigFileChange) {}
func (f fakeConfigFile) AddChangeListenerWithChannel() <-chan model.ConfigFileChangeEvent {
	return make(chan model.ConfigFileChangeEvent)
}

// TestAddressesConfigureTheConfigConnector pins the fix for a bug that was
// silent: NewDefaultConfiguration only writes global.serverConnector.addresses,
// the *discovery* connector, while the config connector keeps its own default of
// 127.0.0.1:8093. Passing the caller's addresses to it therefore configured
// discovery and left every configuration fetch pointed at localhost — failing as
// a timeout that names no cause, with the configured address sitting right there
// in the config file.
func TestAddressesConfigureTheConfigConnector(t *testing.T) {
	var captured polarisconfig.Configuration
	original := newConfigFileAPI
	newConfigFileAPI = func(cfg polarisconfig.Configuration) (api.ConfigFileAPI, error) {
		captured = cfg
		return &fakeConfigFileAPI{}, nil
	}
	defer func() { newConfigFileAPI = original }()

	const addr = "polaris.infra-devops.svc.cluster.local:8093"
	if _, err := New(Options{
		Addresses: []string{addr},
		Namespace: "edu-onex",
		FileGroup: "ox-iam-apiserver",
		FileName:  "ox-iam-apiserver.yaml",
		Token:     "secret-token",
	}); err != nil {
		t.Fatalf("New failed: %v", err)
	}

	connector := captured.GetConfigFile().GetConfigConnectorConfig()
	if got := connector.GetAddresses(); len(got) != 1 || got[0] != addr {
		t.Fatalf("config connector addresses = %v, want [%s]: the fetches would go to the SDK default instead", got, addr)
	}
	if got := connector.GetToken(); got != "secret-token" {
		t.Fatalf("config connector token = %q, want it propagated", got)
	}

	// The discovery connector is a different connector with a different port and
	// is not what this source talks to; it must not have been given the config
	// address.
	if got := captured.GetGlobal().GetServerConnector().GetAddresses(); len(got) == 1 && got[0] == addr {
		t.Fatalf("discovery connector was given the config address %q; the two ports are not interchangeable", addr)
	}
}

// TestLoadFetchesWithoutSubscribing checks that a one-shot load does not start a
// subscription. Load is called on every configuration reload, and each call that
// subscribed would leave a subscription behind that nothing releases.
func TestLoadFetchesWithoutSubscribing(t *testing.T) {
	fake := &fakeConfigFileAPI{content: "level: debug\n"}

	original := newConfigFileAPI
	newConfigFileAPI = func(polarisconfig.Configuration) (api.ConfigFileAPI, error) { return fake, nil }
	defer func() { newConfigFileAPI = original }()

	src, err := New(Options{Namespace: "ns", FileGroup: "g", FileName: "f.yaml"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	kvs, err := src.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if fake.lastRequest == nil {
		t.Fatal("the SDK was never asked for the file")
	}
	if fake.lastRequest.Subscribe {
		t.Error("Load subscribed; a one-shot load must not leave a subscription behind")
	}
	if len(kvs) != 1 || kvs[0].Key != "f.yaml" || string(kvs[0].Value) != "level: debug\n" {
		t.Fatalf("Load returned %+v", kvs)
	}
	if kvs[0].Format != "yaml" {
		t.Errorf("format = %q, want yaml as the default", kvs[0].Format)
	}
}

func TestLoadPropagatesErrors(t *testing.T) {
	fake := &fakeConfigFileAPI{err: errors.New("config file not found")}

	original := newConfigFileAPI
	newConfigFileAPI = func(polarisconfig.Configuration) (api.ConfigFileAPI, error) { return fake, nil }
	defer func() { newConfigFileAPI = original }()

	src, err := New(Options{Namespace: "ns", FileGroup: "g", FileName: "f.yaml"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if _, err := src.Load(); err == nil {
		t.Fatal("expected the fetch error to reach the caller")
	}
}

// TestDestroyIsIdempotentWithNoContext guards the shutdown path: a source whose
// constructor never produced a context must not panic when it is released.
func TestDestroyIsIdempotentWithNoContext(t *testing.T) {
	s := &source{}
	s.Destroy()
	s.Destroy()
}
