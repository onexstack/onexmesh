// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package polaris implements a config.Source backed by the Polaris config
// center (github.com/polarismesh/polaris-go).
package polaris

import (
	"errors"
	"sync"

	"github.com/polarismesh/polaris-go/api"
	polarisconfig "github.com/polarismesh/polaris-go/pkg/config"
	"github.com/polarismesh/polaris-go/pkg/model"

	"github.com/onexstack/onexmesh/pkg/config"
)

// Options configures the Polaris config center Source.
type Options struct {
	// Addresses are the Polaris config center addresses, e.g. ["10.0.0.1:8093"].
	//
	// These are the *config* connector's addresses. They are not the service
	// discovery addresses (8091) the registry backend uses: a Polaris server
	// listens for config on its own port, 8093 by default.
	Addresses []string
	// Namespace is the config file namespace.
	Namespace string
	// FileGroup is the config file group.
	FileGroup string
	// FileName is the config file name.
	FileName string
	// Format is the config file format; empty defaults to yaml.
	Format string
	// Token is the access token the config connector authenticates with. Empty
	// means no token is sent, which is what a server without auth expects.
	Token string
}

func init() {
	config.RegisterSource("polaris", func(opts any) (config.Source, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, errors.New("polaris: options must be polaris.Options")
		}
		return New(o)
	})
}

// source reads a single Polaris config file.
type source struct {
	opts Options
	api  api.ConfigFileAPI
}

// newConfigFileAPI is the SDK constructor, indirected through a variable so
// tests can substitute a fake and exercise Load/Watch without a running Polaris.
var newConfigFileAPI = api.NewConfigFileAPIByConfig

// New creates a Polaris config Source.
//
// The addresses are applied to the *config* connector. This matters because
// NewDefaultConfiguration only writes global.serverConnector.addresses — the
// service-discovery connector — while config.configConnector.addresses carries
// its own default of 127.0.0.1:8093 (pkg/config/default.go
// DefaultConfigConnectorAddresses). Passing the addresses to
// NewDefaultConfiguration therefore configured discovery and left every config
// fetch pointed at localhost, which fails as a timeout that names no cause.
//
// NewDefaultConfiguration(nil) is used rather than the WithDomain variant: the
// latter loads ./polaris.yaml from the working directory when one happens to
// exist, so the effective configuration would depend on where the process was
// started.
func New(opts Options) (config.Source, error) {
	cfg := polarisconfig.NewDefaultConfiguration(nil)

	connector := cfg.GetConfigFile().GetConfigConnectorConfig()
	if len(opts.Addresses) > 0 {
		connector.SetAddresses(opts.Addresses)
	}
	if opts.Token != "" {
		connector.SetToken(opts.Token)
	}

	fileAPI, err := newConfigFileAPI(cfg)
	if err != nil {
		return nil, err
	}
	return &source{opts: opts, api: fileAPI}, nil
}

// Destroy releases the SDK context this source created, including its
// connections and its background tasks. It is not part of config.Source
// because most sources hold nothing to release; callers reach it through a type
// assertion. A caller that reloads configuration must call it on the source it
// is replacing, or every reload leaks a context.
func (s *source) Destroy() {
	if s.api == nil {
		return
	}
	if ctx := s.api.SDKContext(); ctx != nil && !ctx.IsDestroyed() {
		ctx.Destroy()
	}
}

func (s *source) Load() ([]*config.KeyValue, error) {
	// FetchConfigFile with Subscribe false, rather than the GetConfigFile
	// shorthand: the shorthand forces Subscribe true, which would start a
	// subscription that Load's caller never asked for and never releases. A
	// reload that calls Load repeatedly would accumulate one subscription per
	// reload. Watch() subscribes explicitly.
	file, err := s.api.FetchConfigFile(&api.GetConfigFileRequest{
		GetConfigFileRequest: &model.GetConfigFileRequest{
			Namespace: s.opts.Namespace,
			FileGroup: s.opts.FileGroup,
			FileName:  s.opts.FileName,
			Subscribe: false,
		},
	})
	if err != nil {
		return nil, err
	}
	return []*config.KeyValue{{
		Key:    s.opts.FileName,
		Value:  []byte(file.GetContent()),
		Format: s.format(),
	}}, nil
}

func (s *source) Watch() (config.Watcher, error) {
	// Subscribe true: unlike Load, watching is the whole point here.
	file, err := s.api.FetchConfigFile(&api.GetConfigFileRequest{
		GetConfigFileRequest: &model.GetConfigFileRequest{
			Namespace: s.opts.Namespace,
			FileGroup: s.opts.FileGroup,
			FileName:  s.opts.FileName,
			Subscribe: true,
		},
	})
	if err != nil {
		return nil, err
	}
	return &watcher{
		ch:     file.AddChangeListenerWithChannel(),
		key:    s.opts.FileName,
		format: s.format(),
		stopCh: make(chan struct{}),
	}, nil
}

func (s *source) format() string {
	if s.opts.Format != "" {
		return s.opts.Format
	}
	return "yaml"
}

// watcher adapts a Polaris config change channel to config.Watcher.
type watcher struct {
	ch     <-chan model.ConfigFileChangeEvent
	key    string
	format string

	stopOnce sync.Once
	stopCh   chan struct{}
}

func (w *watcher) Next() ([]*config.KeyValue, error) {
	select {
	case <-w.stopCh:
		return nil, errors.New("polaris: config watch stopped")
	case event, ok := <-w.ch:
		if !ok {
			return nil, errors.New("polaris: config watch closed")
		}
		return []*config.KeyValue{{
			Key:    w.key,
			Value:  []byte(event.NewValue),
			Format: w.format,
		}}, nil
	}
}

func (w *watcher) Stop() error {
	// The SDK owns the underlying channel; closing our stopCh unblocks any
	// goroutine blocked in Next so the watch loop can exit.
	w.stopOnce.Do(func() { close(w.stopCh) })
	return nil
}
