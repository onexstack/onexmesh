// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package matcher provides route-level middleware selection: it maps a
// transport operation (gRPC full method or "METHOD /path") to the middlewares
// that apply to it, so different endpoints can carry different cross-cutting
// concerns (e.g. rate limiting only under /admin/*). It is inspired by kratos'
// internal matcher and is transport-agnostic.
package matcher

import (
	"sort"
	"strings"

	"github.com/onexstack/onexmesh/pkg/middleware"
)

// Matcher selects middlewares by operation. Use registers global defaults;
// Add registers middlewares for a selector. Selectors are either an exact
// operation ("/pkg.Service/Method"), a prefix ending in "*" ("/pkg.Service/*"),
// or "*" for every operation.
type Matcher struct {
	defaults []middleware.Middleware
	exact    map[string][]middleware.Middleware
	prefixes map[string][]middleware.Middleware
}

// New returns an empty Matcher.
func New() *Matcher {
	return &Matcher{
		exact:    map[string][]middleware.Middleware{},
		prefixes: map[string][]middleware.Middleware{},
	}
}

// Use appends global middlewares applied to every operation, outermost first.
func (m *Matcher) Use(mws ...middleware.Middleware) *Matcher {
	m.defaults = append(m.defaults, mws...)
	return m
}

// Add registers middlewares for a selector, returning m for chaining.
func (m *Matcher) Add(selector string, mws ...middleware.Middleware) *Matcher {
	switch {
	case selector == "*":
		m.prefixes[""] = append(m.prefixes[""], mws...)
	case strings.HasSuffix(selector, "*"):
		prefix := strings.TrimSuffix(selector, "*")
		m.prefixes[prefix] = append(m.prefixes[prefix], mws...)
	default:
		m.exact[selector] = append(m.exact[selector], mws...)
	}
	return m
}

// Match returns the middlewares applying to operation: defaults first, then an
// exact match, then prefix matches from longest to shortest prefix.
func (m *Matcher) Match(operation string) []middleware.Middleware {
	ms := make([]middleware.Middleware, 0, len(m.defaults)+4)
	ms = append(ms, m.defaults...)

	if exact, ok := m.exact[operation]; ok {
		ms = append(ms, exact...)
	}

	var matched []string
	for prefix := range m.prefixes {
		if strings.HasPrefix(operation, prefix) {
			matched = append(matched, prefix)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return len(matched[i]) > len(matched[j]) })
	for _, prefix := range matched {
		ms = append(ms, m.prefixes[prefix]...)
	}

	return ms
}
