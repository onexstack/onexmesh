// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package generators holds the code generators that produce the client-go
// style REST SDK. Each generator is a pure function (func(...) string).
package generators

import (
	"fmt"
	"go/format"
)

// FormatSource runs gofmt (including import sorting) over generated content.
//
// protogen's GeneratedFile.Content() already reformats via a printer, but it
// does NOT sort imports (ast.SortImports is a gofmt-command-only step), so
// generated import blocks can end up unsorted. This is idempotent: the
// subsequent printer pass preserves the already-sorted order.
//
// It is exported so that both the plugin entrypoint (write) and the golden
// regression tests can gofmt the exact same bytes that land on disk.
func FormatSource(filename, content string) (string, error) {
	formatted, err := format.Source([]byte(content))
	if err != nil {
		return "", fmt.Errorf("%s: generated source is not valid Go: %w", filename, err)
	}
	return string(formatted), nil
}
