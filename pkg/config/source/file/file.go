// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package file implements a local-file config.Source with polling-based
// hot-reload.
package file

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/onexstack/onexmesh/pkg/config"
)

const pollInterval = time.Second

func init() {
	config.RegisterSource("file", func(opts any) (config.Source, error) {
		paths, ok := opts.([]string)
		if !ok {
			return nil, errors.New("file: options must be []string of paths")
		}
		return New(paths...), nil
	})
}

// source reads configuration from one or more local files.
type source struct {
	paths []string
}

// New returns a Source reading the given file paths.
func New(paths ...string) config.Source {
	return &source{paths: paths}
}

func (s *source) Load() ([]*config.KeyValue, error) {
	var kvs []*config.KeyValue
	for _, p := range s.paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		kvs = append(kvs, &config.KeyValue{
			Key:    p,
			Value:  data,
			Format: formatFromExt(p),
		})
	}
	return kvs, nil
}

func (s *source) Watch() (config.Watcher, error) {
	w := &watcher{
		paths:   s.paths,
		lastMod: map[string]time.Time{},
		stop:    make(chan struct{}),
	}
	for _, p := range s.paths {
		if info, err := os.Stat(p); err == nil {
			w.lastMod[p] = info.ModTime()
		}
	}
	return w, nil
}

// watcher polls file mtimes and emits a snapshot on change.
type watcher struct {
	paths   []string
	lastMod map[string]time.Time
	stop    chan struct{}
}

func (w *watcher) Next() ([]*config.KeyValue, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return nil, errors.New("file: watcher stopped")
		case <-ticker.C:
			if kvs, changed := w.check(); changed {
				return kvs, nil
			}
		}
	}
}

func (w *watcher) Stop() error {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	return nil
}

func (w *watcher) check() ([]*config.KeyValue, bool) {
	var kvs []*config.KeyValue
	changed := false
	for _, p := range w.paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if prev, ok := w.lastMod[p]; !ok || !prev.Equal(info.ModTime()) {
			w.lastMod[p] = info.ModTime()
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			kvs = append(kvs, &config.KeyValue{Key: p, Value: data, Format: formatFromExt(p)})
			changed = true
		}
	}
	return kvs, changed
}

func formatFromExt(p string) string {
	switch filepath.Ext(p) {
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	default:
		return "json"
	}
}
