// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package rest

import (
	"strings"

	meshclient "github.com/onexstack/onexmesh/pkg/client"
)

// SetDefaultRegistry forwards to meshclient.SetDefaultRegistry so REST callers
// can configure the process-wide default registry without importing the gRPC
// client package.
func SetDefaultRegistry(name string, opts any) {
	meshclient.SetDefaultRegistry(name, opts)
}

// MeshServiceName reports the service name when host is a onexmesh:// reference
// (e.g. "onexmesh://mb-ginserver" -> "mb-ginserver"), and ok=false otherwise.
func MeshServiceName(host string) (string, bool) {
	prefix := meshclient.Scheme + "://"
	if !strings.HasPrefix(host, prefix) {
		return "", false
	}
	return strings.TrimPrefix(host, prefix), true
}
