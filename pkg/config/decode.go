// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config

import (
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

// DecodeOptions overlays configuration-file values onto a backend's typed
// Options.
//
// It exists because a backend's typed Options is deliberately not a field of the
// option struct that owns the section: that is the dependency the backend
// registry is there to avoid. So the values a file carries under, say,
// config-source.polaris.* have nowhere to land unless the backend is handed them
// and decodes them itself. This is the single implementation behind every
// backend's Decode, so a backend gains file configuration with one line rather
// than a hand-written field mapping per option.
//
// The matcher ignores case and hyphens, so the key "file-group" and the Go field
// FileGroup are the same option. Without that every multi-word option would
// decode to nothing and leave its default in place — a failure with no symptom,
// since the service starts and merely ignores what it was configured with. That
// is the same shape of mistake this layer exists to remove.
//
// The decoder config mirrors viper's, so a value written in a config file
// behaves here as it does everywhere else in the application: a number or a
// boolean may arrive as a string, and a duration may be written "10s".
func DecodeOptions(raw map[string]any, out any) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           out,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
		MatchName: func(mapKey, fieldName string) bool {
			return normalizeKey(mapKey) == normalizeKey(fieldName)
		},
	})
	if err != nil {
		return err
	}
	return dec.Decode(raw)
}

// normalizeKey folds a key to the form DecodeOptions' matcher compares on:
// case-insensitive and hyphen-blind, so "file-group", "filegroup" and FileGroup
// all agree.
func normalizeKey(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "-", ""))
}
