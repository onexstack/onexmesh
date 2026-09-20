// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package static

import (
	"context"
	"strings"
	"testing"
	"time"
)

const svc = "edu.onex.commerce-apiserver"

func TestParseBindings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      []string
		want    map[string][]string
		wantErr string
	}{
		{
			name: "one service, one address",
			in:   []string{svc + "=http://127.0.0.1:8182"},
			want: map[string][]string{svc: {"http://127.0.0.1:8182"}},
		},
		{
			name: "a repeated service accumulates addresses in order",
			in: []string{
				svc + "=http://127.0.0.1:8182",
				svc + "=http://127.0.0.1:8183",
			},
			want: map[string][]string{svc: {"http://127.0.0.1:8182", "http://127.0.0.1:8183"}},
		},
		{
			name: "two services do not bleed into each other",
			in: []string{
				svc + "=http://127.0.0.1:8182",
				"edu.onex.catalog-apiserver=127.0.0.1:8181",
			},
			want: map[string][]string{
				svc:                          {"http://127.0.0.1:8182"},
				"edu.onex.catalog-apiserver": {"127.0.0.1:8181"},
			},
		},
		{
			name: "surrounding whitespace is trimmed",
			in:   []string{" " + svc + " = http://127.0.0.1:8182 "},
			want: map[string][]string{svc: {"http://127.0.0.1:8182"}},
		},
		{
			name:    "no separator is refused, not skipped",
			in:      []string{"http://127.0.0.1:8182"},
			wantErr: "not in service=address form",
		},
		{
			name:    "an empty service is refused",
			in:      []string{"=http://127.0.0.1:8182"},
			wantErr: "names no service",
		},
		{
			name:    "an empty address is refused",
			in:      []string{svc + "="},
			wantErr: "is bound to no address",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBindings(tc.in)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("want an error containing %q, got nil", tc.wantErr)
				}
				if !contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for k, want := range tc.want {
				gotAddrs, ok := got[k]
				if !ok {
					t.Fatalf("missing service %q in %v", k, got)
				}
				if len(gotAddrs) != len(want) {
					t.Fatalf("%s: got %v, want %v", k, gotAddrs, want)
				}
				for i := range want {
					if gotAddrs[i] != want[i] {
						t.Fatalf("%s[%d]: got %q, want %q", k, i, gotAddrs[i], want[i])
					}
				}
			}
		})
	}
}

// TestParseBindingsRoundTrip pins formatBindings against ParseBindings, so the
// two cannot drift into disagreeing about what a binding means.
func TestParseBindingsRoundTrip(t *testing.T) {
	original := map[string][]string{
		svc:                          {"http://127.0.0.1:8182", "http://127.0.0.1:8183"},
		"edu.onex.catalog-apiserver": {"127.0.0.1:8181"},
	}
	got, err := ParseBindings(formatBindings(original))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(original) {
		t.Fatalf("got %v, want %v", got, original)
	}
	for k, want := range original {
		if len(got[k]) != len(want) {
			t.Fatalf("%s: got %v, want %v", k, got[k], want)
		}
		for i := range want {
			if got[k][i] != want[i] {
				t.Fatalf("%s[%d]: got %q, want %q", k, i, got[k][i], want[i])
			}
		}
	}
}

func TestNewDiscoveryRefusesMisconfigurationAtConstruction(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    Options
		wantErr string
	}{
		{name: "no endpoints at all", opts: Options{}, wantErr: "no endpoints configured"},
		{
			name:    "a service bound to nothing",
			opts:    Options{Endpoints: map[string][]string{svc: {}}},
			wantErr: "is bound to no address",
		},
		{
			name:    "an entry with no service name",
			opts:    Options{Endpoints: map[string][]string{"": {"http://127.0.0.1:8182"}}},
			wantErr: "bound to no service name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDiscovery(tc.opts)
			if err == nil {
				t.Fatal("want an error, got nil")
			}
			if !contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestGetService(t *testing.T) {
	d, err := NewDiscovery(Options{Endpoints: map[string][]string{
		svc: {"http://127.0.0.1:8182", "https://commerce.internal:443"},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instances, err := d.GetService(context.Background(), svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("got %d instances, want 2", len(instances))
	}
	// Each address is one instance carrying one endpoint: the round tripper
	// parses "scheme://host:port" out of Endpoints, so wrapping both addresses in
	// a single instance would make one of them unreachable.
	for i, want := range []string{"http://127.0.0.1:8182", "https://commerce.internal:443"} {
		if len(instances[i].Endpoints) != 1 {
			t.Fatalf("instance %d has %d endpoints, want 1", i, len(instances[i].Endpoints))
		}
		if instances[i].Endpoints[0] != want {
			t.Fatalf("instance %d endpoint: got %q, want %q", i, instances[i].Endpoints[0], want)
		}
		if instances[i].Name != svc {
			t.Fatalf("instance %d name: got %q, want %q", i, instances[i].Name, svc)
		}
	}
}

// TestGetServiceForAnUnknownNameIsAnError pins the distinction the package doc
// claims: an unconfigured service is not the same fact as a service with no
// instances, so it must not come back as an empty list.
func TestGetServiceForAnUnknownNameIsAnError(t *testing.T) {
	d, err := NewDiscovery(Options{Endpoints: map[string][]string{svc: {"http://127.0.0.1:8182"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instances, err := d.GetService(context.Background(), "edu.onex.typo-apiserver")
	if err == nil {
		t.Fatalf("want an error, got %d instances", len(instances))
	}
	if !contains(err.Error(), "not configured") {
		t.Fatalf("error %q does not say the service is unconfigured", err.Error())
	}
	// The message names what is configured, so a typo can be seen without
	// opening the config file.
	if !contains(err.Error(), svc) {
		t.Fatalf("error %q does not list the known services", err.Error())
	}
}

// TestWatchBlocksRatherThanCompleting pins the one behaviour the gRPC resolver
// depends on: a static list never emits a snapshot, because an empty snapshot
// reads as "no healthy instances" and would drop the addresses a client is
// already using.
func TestWatchBlocksRatherThanCompleting(t *testing.T) {
	d, err := NewDiscovery(Options{Endpoints: map[string][]string{svc: {"http://127.0.0.1:8182"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w, err := d.Watch(ctx, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type result struct {
		instances int
		err       error
	}
	done := make(chan result, 1)
	go func() {
		instances, err := w.Next()
		done <- result{len(instances), err}
	}()

	select {
	case r := <-done:
		t.Fatalf("Next returned before the context was cancelled: %d instances, err=%v", r.instances, r.err)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
	select {
	case r := <-done:
		if r.instances != 0 {
			t.Fatalf("Next reported %d instances after cancellation, want 0", r.instances)
		}
		if r.err == nil {
			t.Fatal("Next returned nil after cancellation; the caller cannot tell why it stopped")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Next did not return after its context was cancelled")
	}

	if err := w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestTheMapIsCopied pins that a caller mutating its own map after construction
// does not change what the discovery serves.
func TestTheMapIsCopied(t *testing.T) {
	opts := Options{Endpoints: map[string][]string{svc: {"http://127.0.0.1:8182"}}}
	d, err := NewDiscovery(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opts.Endpoints[svc][0] = "http://127.0.0.1:9999"
	opts.Endpoints["edu.onex.sneaky-apiserver"] = []string{"http://127.0.0.1:9999"}

	instances, err := d.GetService(context.Background(), svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instances[0].Endpoints[0] != "http://127.0.0.1:8182" {
		t.Fatalf("the discovery followed the caller's later mutation: %q", instances[0].Endpoints[0])
	}
	if _, err := d.GetService(context.Background(), "edu.onex.sneaky-apiserver"); err == nil {
		t.Fatal("a service added to the caller's map after construction is being served")
	}
}

// contains reports whether haystack has needle in it, for asserting on error
// text without pinning the whole sentence.
func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
