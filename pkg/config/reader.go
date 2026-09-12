// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// reader stores decoded configuration as a nested map and provides dot-path
// random access. It is safe for concurrent use.
type reader struct {
	mu     sync.RWMutex
	values map[string]any
}

func newReader() *reader {
	return &reader{values: map[string]any{}}
}

// Merge decodes each raw KeyValue and deep-merges it onto the current map.
func (r *reader) Merge(kvs ...*KeyValue) error {
	for _, kv := range kvs {
		var data map[string]any
		if err := decode(kv.Value, kv.Format, &data); err != nil {
			return fmt.Errorf("config: decode %q: %w", kv.Key, err)
		}
		r.mu.Lock()
		mergeMap(r.values, data)
		r.mu.Unlock()
	}
	return nil
}

// mergeMap recursively overlays src onto dst. Nested maps are merged key by
// key so later sources override only the keys they define; everything else is
// replaced wholesale.
func mergeMap(dst, src map[string]any) {
	for k, sv := range src {
		dv, ok := dst[k]
		if !ok {
			dst[k] = sv
			continue
		}
		srcMap, srcIsMap := sv.(map[string]any)
		dstMap, dstIsMap := dv.(map[string]any)
		if srcIsMap && dstIsMap {
			mergeMap(dstMap, srcMap)
			continue
		}
		dst[k] = sv
	}
}

func (r *reader) Value(path string) Value {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return newValue(deepCopyValue(lookup(r.values, path)))
}

// deepCopyValue returns a deep copy of a decoded configuration value so that a
// caller holding a Value never observes a concurrent merge from a background
// Watch. It copies map[string]any and []any recursively; other values are
// immutable and returned as-is.
func deepCopyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			m[k] = deepCopyValue(vv)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, vv := range t {
			s[i] = deepCopyValue(vv)
		}
		return s
	default:
		return v
	}
}

func (r *reader) Source() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return json.Marshal(r.values)
}

// lookup walks a dot-separated path through a nested map.
func lookup(m map[string]any, path string) any {
	if path == "" {
		return m
	}
	var cur any = m
	for _, part := range strings.Split(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = mm[part]
		if !ok {
			return nil
		}
	}
	return cur
}

// decode unmarshals raw bytes according to the declared format. An empty
// format defaults to JSON.
func decode(data []byte, format string, out *map[string]any) error {
	switch format {
	case "yaml", "yml":
		return yaml.Unmarshal(data, out)
	case "toml":
		return toml.Unmarshal(data, out)
	default:
		return json.Unmarshal(data, out)
	}
}
