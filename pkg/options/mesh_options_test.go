// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"strings"
	"testing"
)

// TestMeshOptionsAdvertiseHost pins the fix for the bug this replaced: the
// registrar was built from the *listen* address, so a service listening on
// 0.0.0.0:8180 published "0.0.0.0:8180" to the registry — an entry that
// resolves to whoever is dialing it. Every case below returns an address a peer
// could actually call.
func TestMeshOptionsAdvertiseHost(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		podIP   string
		want    string // exact match when set
		wantAny bool   // otherwise any non-empty, non-wildcard result passes
	}{
		{
			name:  "explicit host wins",
			host:  "10.0.0.7",
			podIP: "10.0.0.9",
			want:  "10.0.0.7",
		},
		{
			name: "explicit host may be a name",
			host: "svc.internal",
			want: "svc.internal",
		},
		{
			name:  "wildcard host falls through to POD_IP",
			host:  "0.0.0.0",
			podIP: "10.0.0.9",
			want:  "10.0.0.9",
		},
		{
			name:  "IPv6 wildcard falls through to POD_IP",
			host:  "::",
			podIP: "10.0.0.9",
			want:  "10.0.0.9",
		},
		{
			// The case that matters in a container: nothing configured, POD_IP
			// injected by the downward API.
			name:  "POD_IP used when nothing is configured",
			podIP: "10.0.0.9",
			want:  "10.0.0.9",
		},
		{
			name:    "probed when neither is set",
			wantAny: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setenv with an empty value still sets the variable, which is
			// what a Pod spec without a downward API entry does not do — so
			// clear it explicitly in that case.
			if tt.podIP == "" {
				t.Setenv("POD_IP", "")
			} else {
				t.Setenv("POD_IP", tt.podIP)
			}

			o := NewMeshOptions()
			o.Host = tt.host

			got, err := o.AdvertiseHost()
			if err != nil {
				t.Fatalf("AdvertiseHost() error = %v", err)
			}
			if tt.want != "" && got != tt.want {
				t.Fatalf("AdvertiseHost() = %q, want %q", got, tt.want)
			}
			if tt.wantAny && got == "" {
				t.Fatal("AdvertiseHost() = empty, want a probed address")
			}
			// No path may publish a wildcard: that is the original bug.
			if isUnspecifiedHost(got) {
				t.Fatalf("AdvertiseHost() = %q, which is a wildcard a peer cannot dial", got)
			}
			if strings.TrimSpace(got) == "" {
				t.Fatal("AdvertiseHost() = blank")
			}
		})
	}
}

// TestMeshOptionsPortDefaultsToListen pins the companion decision: the
// registered port comes from the listen address unless it is set explicitly,
// so a default here would be a second source of truth.
func TestMeshOptionsPortDefaultsToListen(t *testing.T) {
	if got := NewMeshOptions().Port; got != 0 {
		t.Fatalf("NewMeshOptions().Port = %d, want 0 (meaning: use the listened port)", got)
	}
}
