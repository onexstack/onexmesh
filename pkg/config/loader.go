// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config

// loader loads raw configuration from sources into a reader.
type loader struct {
	reader Reader
}

func newLoader(reader Reader) *loader {
	return &loader{reader: reader}
}

// Load loads all sources and merges their key/values into the reader.
func (l *loader) Load(sources ...Source) error {
	var kvs []*KeyValue
	for _, s := range sources {
		sks, err := s.Load()
		if err != nil {
			return err
		}
		kvs = append(kvs, sks...)
	}
	return l.reader.Merge(kvs...)
}

// Snapshot returns a point-in-time snapshot of the merged configuration.
func (l *loader) Snapshot() (*Snapshot, error) {
	data, err := l.reader.Source()
	if err != nil {
		return nil, err
	}
	return &Snapshot{data: data}, nil
}
