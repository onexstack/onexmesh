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
	// Addresses are the Polaris server addresses, e.g. ["127.0.0.1:8093"].
	Addresses []string
	// Namespace is the config file namespace.
	Namespace string
	// FileGroup is the config file group.
	FileGroup string
	// FileName is the config file name.
	FileName string
	// Format is the config file format; empty defaults to yaml.
	Format string
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

// New creates a Polaris config Source.
func New(opts Options) (config.Source, error) {
	cfg := polarisconfig.NewDefaultConfiguration(opts.Addresses)
	fileAPI, err := api.NewConfigFileAPIByConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &source{opts: opts, api: fileAPI}, nil
}

func (s *source) Load() ([]*config.KeyValue, error) {
	file, err := s.api.GetConfigFile(s.opts.Namespace, s.opts.FileGroup, s.opts.FileName)
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
	file, err := s.api.GetConfigFile(s.opts.Namespace, s.opts.FileGroup, s.opts.FileName)
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
