// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package options defines the configuration option structs for onexmesh,
// following the onex IOptions convention: each option implements Validate and
// AddFlags (with a full prefix), and is combined into a ServerOptions root.
package options

import "github.com/spf13/pflag"

// IOptions is implemented by every option struct. AddFlags receives a full
// prefix (e.g. "mesh", "otel") and appends its own field names to build flags
// like --otel.endpoint.
type IOptions interface {
	// Validate validates all required options, returning all errors.
	Validate() []error
	// AddFlags registers flags with the given full prefix.
	AddFlags(fs *pflag.FlagSet, fullPrefix string)
}
