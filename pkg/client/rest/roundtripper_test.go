// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// fakeDiscovery is an in-memory Discovery returning a fixed instance set.
type fakeDiscovery struct {
	instances []*registry.ServiceInstance
}

func (d *fakeDiscovery) GetService(ctx context.Context, name string) ([]*registry.ServiceInstance, error) {
	return d.instances, nil
}

func (d *fakeDiscovery) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	return nil, nil
}

func (d *fakeDiscovery) Close() error { return nil }

func TestRoundTripRewritesHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "host=%s", r.Host)
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	rt, err := newMeshRoundTripper("svc", WithDiscovery(&fakeDiscovery{instances: []*registry.ServiceInstance{{
		ID:        "i1",
		Name:      "svc",
		Endpoints: []string{"http://" + u.Host},
	}}}))
	if err != nil {
		t.Fatal(err)
	}
	rt.base = http.DefaultTransport

	req := httptest.NewRequest(http.MethodGet, placeholderHost+"/apis/apps/v1/deployments", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "host="+u.Host; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestRoundTripFiltersGRPCEndpoints(t *testing.T) {
	rt, err := newMeshRoundTripper("svc", WithDiscovery(&fakeDiscovery{instances: []*registry.ServiceInstance{{
		ID:        "i1",
		Name:      "svc",
		Endpoints: []string{"grpc://127.0.0.1:39090"},
	}}}))
	if err != nil {
		t.Fatal(err)
	}
	rt.base = http.DefaultTransport

	req := httptest.NewRequest(http.MethodGet, placeholderHost+"/x", nil)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("expected error when no http(s) instance is present")
	}
}
