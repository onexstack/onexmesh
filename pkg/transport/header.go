// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package transport

import (
	"net/http"
	"sort"

	"google.golang.org/grpc/metadata"
)

// Header is a protocol-agnostic view over request/response headers. It is
// satisfied by both http.Header and grpc metadata.MD through the adapters
// below, so middleware can read and write headers without knowing the transport.
type Header interface {
	// Get returns the first value for key, or "" when absent.
	Get(key string) string
	// Set replaces the value for key with a single value.
	Set(key, value string)
	// Add appends value to key.
	Add(key, value string)
	// Keys returns all present header keys.
	Keys() []string
	// Values returns all values for key.
	Values(key string) []string
}

// httpHeader adapts net/http.Header to the Header interface.
type httpHeader struct {
	http.Header
}

// NewHTTPHeader wraps an http.Header as a Header.
func NewHTTPHeader(h http.Header) Header { return httpHeader{h} }

func (h httpHeader) Get(key string) string      { return h.Header.Get(key) }
func (h httpHeader) Set(key, value string)      { h.Header.Set(key, value) }
func (h httpHeader) Add(key, value string)      { h.Header.Add(key, value) }
func (h httpHeader) Values(key string) []string { return h.Header.Values(key) }
func (h httpHeader) Keys() []string {
	keys := make([]string, 0, len(h.Header))
	for k := range h.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// metadataHeader adapts grpc metadata.MD to the Header interface.
type metadataHeader struct {
	metadata.MD
}

// NewMetadataHeader wraps a grpc metadata.MD as a Header.
func NewMetadataHeader(md metadata.MD) Header { return metadataHeader{md} }

// Get returns the first value for key. Unlike metadata.MD.Get, it returns ""
// instead of panicking when the key is absent.
func (h metadataHeader) Get(key string) string {
	vs := h.MD.Get(key)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

func (h metadataHeader) Set(key, value string)      { h.MD.Set(key, value) }
func (h metadataHeader) Add(key, value string)      { h.MD.Append(key, value) }
func (h metadataHeader) Values(key string) []string { return h.MD.Get(key) }
func (h metadataHeader) Keys() []string {
	keys := make([]string, 0, len(h.MD))
	for k := range h.MD {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
