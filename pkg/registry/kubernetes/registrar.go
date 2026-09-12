// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kubernetes

import (
	"context"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// registrar is a no-op: in Kubernetes, instance membership is declared through
// a Service selector and reconciled by the endpoint controller, so the SDK
// performs no explicit registration.
type registrar struct{}

// NewRegistrar creates a no-op Kubernetes Registrar.
func NewRegistrar(opts Options) (registry.Registrar, error) {
	return &registrar{}, nil
}

func (r *registrar) Register(ctx context.Context, inst *registry.ServiceInstance) error {
	return nil
}

func (r *registrar) Deregister(ctx context.Context, inst *registry.ServiceInstance) error {
	return nil
}
