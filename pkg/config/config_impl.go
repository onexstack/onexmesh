// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config

import (
	"encoding/json"
	"sync"
)

// Option configures a Config.
type Option func(*config)

// WithSource appends a configuration Source.
func WithSource(s Source) Option {
	return func(c *config) { c.sources = append(c.sources, s) }
}

// config is the default Config implementation combining sources, a loader and
// a reader.
type config struct {
	sources []Source
	reader  Reader
	loader  *loader

	mu       sync.Mutex
	watchers []Watcher
	closed   bool
}

// New builds a Config from the given options.
func New(opts ...Option) Config {
	r := newReader()
	c := &config{reader: r}
	c.loader = newLoader(r)
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *config) Load() error {
	return c.loader.Load(c.sources...)
}

func (c *config) Value(path string) Value {
	return c.reader.Value(path)
}

func (c *config) Scan(v interface{}) error {
	data, err := c.reader.Source()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (c *config) Reader() Reader {
	return c.reader
}

// Watch registers fn to be called on every configuration change. It spawns one
// watch goroutine per source and returns immediately.
func (c *config) Watch(fn func(Config)) error {
	for _, s := range c.sources {
		w, err := s.Watch()
		if err != nil {
			continue
		}
		c.mu.Lock()
		c.watchers = append(c.watchers, w)
		c.mu.Unlock()
		go c.watchLoop(w, fn)
	}
	return nil
}

func (c *config) watchLoop(w Watcher, fn func(Config)) {
	for {
		kvs, err := w.Next()
		if err != nil {
			return
		}
		if err := c.reader.Merge(kvs...); err != nil {
			continue
		}
		fn(c)
	}
}

// Close stops all watchers and releases resources.
func (c *config) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	for _, w := range c.watchers {
		_ = w.Stop()
	}
	c.watchers = nil
	return nil
}
