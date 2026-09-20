// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package static implements onexmesh registry.Discovery over a fixed address
// list, so a client can talk to a known ip:port without a service registry.
//
// # Why this exists
//
// Every other backend answers "where is service X" by asking something —
// Polaris, etcd, the Kubernetes API. The other half of that question is "I
// already know where it is", and until this package there was no backend for
// it: pkg/client.Dial (gRPC) has an isHostPort branch that dials a literal
// address, but pkg/client/rest has only the discovery path, and the registry
// backends shipped are all external systems. So a REST client could not be
// pointed at 127.0.0.1:8182 at all — with `registry.type: none` the round
// tripper had no discovery and failed with "registry discovery \"none\" is not
// registered".
//
// # Where it is meant to be used
//
// Client side. A service that calls another at a known address builds its
// transport with this backend:
//
//	cfg, _ := meshrest.NewForMeshConfig("edu.onex.commerce-apiserver",
//	    meshrest.WithRegistry("static", static.Options{
//	        Endpoints: map[string][]string{
//	            "edu.onex.commerce-apiserver": {"http://127.0.0.1:8182"},
//	        },
//	    }))
//
// It is a development and single-host-deployment facility, not a production
// service mesh. It has no health checking, no failover and no watching: the
// list is whatever the operator wrote, and a dead address stays in it.
//
// # Registration is a deliberate no-op
//
// NewRegistrar returns a Registrar that does nothing but say so. Setting the
// registry type to "static" means "do not register me anywhere", which is a
// coherent instruction from an operator running a one-host deployment — and the
// failure mode to avoid is the opposite one, where the service believes it is
// registered and no client can find it.
package static
