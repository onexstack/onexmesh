// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package static

import (
	"fmt"
	"sort"
	"strings"
)

// Options configures the static backend.
type Options struct {
	// Endpoints maps a logical service name to the addresses it is reachable at.
	//
	// An address is "host:port", optionally with a scheme: "http://127.0.0.1:8182".
	// Without a scheme the REST transport assumes http, which is what a bare
	// "127.0.0.1:8182" should mean — nobody writes that intending TLS.
	//
	// A service name with no entry here is an error at lookup rather than an
	// empty instance list. The two are different facts — "nobody configured this
	// service" and "the service is configured and has no healthy instances" —
	// and collapsing them would make a typo in a service name look like an
	// outage.
	Endpoints map[string][]string
}

// bindingSeparator separates the service name from an address in a flag
// binding, e.g. "edu.onex.commerce-apiserver=http://127.0.0.1:8182".
const bindingSeparator = "="

// ParseBindings turns the flag form into the map form.
//
// The flag form exists because pflag has no map-of-lists type, so a backend that
// wants to be configurable from the command line as well as from a YAML file
// needs a flat encoding. Each element names the service it belongs to and may be
// repeated to give a service more than one address:
//
//	--registry.static.endpoints=edu.onex.commerce-apiserver=http://127.0.0.1:8182
//	--registry.static.endpoints=edu.onex.commerce-apiserver=http://127.0.0.1:8183
//
// A `service=address` entry with an empty side is refused rather than ignored:
// `=http://a:1` cannot be routed to anything, and `svc=` would register a
// service with no address, which a client would then fail to reach with a
// message about instances rather than about configuration.
func ParseBindings(bindings []string) (map[string][]string, error) {
	out := make(map[string][]string, len(bindings))
	for _, b := range bindings {
		service, addr, ok := strings.Cut(b, bindingSeparator)
		if !ok {
			return nil, fmt.Errorf(
				"static: endpoint %q is not in service=address form (e.g. edu.onex.commerce-apiserver=http://127.0.0.1:8182)", b)
		}
		service = strings.TrimSpace(service)
		addr = strings.TrimSpace(addr)
		if service == "" {
			return nil, fmt.Errorf("static: endpoint %q names no service", b)
		}
		if addr == "" {
			return nil, fmt.Errorf("static: service %q is bound to no address", service)
		}
		out[service] = append(out[service], addr)
	}
	return out, nil
}

// formatBindings is ParseBindings' inverse, for error messages and tests.
func formatBindings(endpoints map[string][]string) []string {
	out := make([]string, 0, len(endpoints))
	for service, addrs := range endpoints {
		for _, addr := range addrs {
			out = append(out, service+bindingSeparator+addr)
		}
	}
	sort.Strings(out)
	return out
}
