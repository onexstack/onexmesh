// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package metadata provides a canonical map[string]string carried across RPC
// and message boundaries, plus helpers to attach and read it from a context.
// It complements pkg/transport's protocol-agnostic Header view: Header adapts
// http.Header / grpc metadata.MD, while Metadata is the normalized key/value
// payload used by broker messages and service-to-service propagation.
package metadata

import "context"

// Metadata is a normalized set of string key/value pairs.
type Metadata map[string]string

// Get returns the value for key and whether it was present.
func (md Metadata) Get(key string) (string, bool) {
	v, ok := md[key]
	return v, ok
}

// Set stores val under key.
func (md Metadata) Set(key, val string) {
	md[key] = val
}

// Delete removes key.
func (md Metadata) Delete(key string) {
	delete(md, key)
}

// Copy returns a shallow copy of md.
func Copy(md Metadata) Metadata {
	out := make(Metadata, len(md))
	for k, v := range md {
		out[k] = v
	}
	return out
}

type metadataKey struct{}

// NewContext returns ctx carrying md.
func NewContext(ctx context.Context, md Metadata) context.Context {
	return context.WithValue(ctx, metadataKey{}, md)
}

// FromContext returns the metadata attached to ctx.
func FromContext(ctx context.Context) (Metadata, bool) {
	md, ok := ctx.Value(metadataKey{}).(Metadata)
	return md, ok
}

// MergeContext returns ctx with patch merged into its existing metadata. When
// overwrite is false, existing values win over patch.
func MergeContext(ctx context.Context, patch Metadata, overwrite bool) context.Context {
	existing, _ := FromContext(ctx)
	merged := Copy(existing)
	if merged == nil {
		merged = Metadata{}
	}
	for k, v := range patch {
		if _, exists := merged[k]; !exists || overwrite {
			merged[k] = v
		}
	}
	return NewContext(ctx, merged)
}

// Set returns ctx with a single metadata key set.
func Set(ctx context.Context, key, val string) context.Context {
	md, _ := FromContext(ctx)
	if md == nil {
		md = Metadata{}
	}
	merged := Copy(md)
	merged[key] = val
	return NewContext(ctx, merged)
}

// Delete returns ctx with a single metadata key removed.
func Delete(ctx context.Context, key string) context.Context {
	md, _ := FromContext(ctx)
	if md == nil {
		return ctx
	}
	merged := Copy(md)
	delete(merged, key)
	return NewContext(ctx, merged)
}

// Get returns the value for key in ctx's metadata.
func Get(ctx context.Context, key string) (string, bool) {
	md, ok := FromContext(ctx)
	if !ok {
		return "", false
	}
	return md.Get(key)
}
