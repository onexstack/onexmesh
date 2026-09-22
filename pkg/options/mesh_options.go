// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
)

var _ IOptions = (*MeshOptions)(nil)

// MeshOptions holds the core service identity and listen addresses.
type MeshOptions struct {
	ServiceName string `mapstructure:"service-name"`
	Protocol    string `mapstructure:"protocol"` // "grpc", "http", or "both"
	GRPCAddr    string `mapstructure:"grpc-addr"`
	HTTPAddr    string `mapstructure:"http-addr"`
	// Host is the address peers reach this instance on, used for registration
	// only; it has nothing to do with what the listener binds. Empty means
	// "work it out" — see AdvertiseHost.
	Host string `mapstructure:"host"`
	// Port overrides the registered port. Zero — the default — registers the
	// port actually listened on, which is almost always what is wanted.
	Port int `mapstructure:"port"`
	// Env names the deployment environment (dev/test/stg/prod). It is published
	// as the "env" instance metadata so one registry namespace can hold several
	// environments without a caller mistaking a developer's laptop for a
	// production replica.
	Env string `mapstructure:"env"`
	// Version is the service version, published so a caller can pin or canary.
	Version string `mapstructure:"version"`
	// Metadata is arbitrary extra instance metadata.
	Metadata map[string]string `mapstructure:"metadata"`
	// MiddlewareRoutes are optional route-level middleware bindings, each of
	// the form "selector=mw1,mw2" (e.g. "/svc.v1.Admin/*=ratelimit").
	MiddlewareRoutes []string `mapstructure:"middleware-route"`
}

// NewMeshOptions returns default mesh options.
//
// Port defaults to 0 rather than to a listening port: the registered port must
// be the one the process actually binds, and a default here would be a second
// source of truth that silently disagrees with GRPCAddr/HTTPAddr.
func NewMeshOptions() *MeshOptions {
	return &MeshOptions{
		Protocol: "grpc",
		GRPCAddr: "0.0.0.0:9090",
		HTTPAddr: "0.0.0.0:8080",
	}
}

func (o *MeshOptions) Validate() []error {
	var errs []error
	if o.ServiceName == "" {
		errs = append(errs, fmt.Errorf("service name is required"))
	}
	switch o.Protocol {
	case "grpc", "http", "both":
	default:
		errs = append(errs, fmt.Errorf("invalid protocol %q", o.Protocol))
	}
	if o.Port < 0 || o.Port > 65535 {
		errs = append(errs, fmt.Errorf("invalid port %d", o.Port))
	}
	return errs
}

func (o *MeshOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&o.ServiceName, prefix+".service-name", o.ServiceName, "Service name, e.g. edu.course.student-api.")
	fs.StringVar(&o.Protocol, prefix+".protocol", o.Protocol, "Protocol: grpc, http, or both.")
	fs.StringVar(&o.GRPCAddr, prefix+".grpc-addr", o.GRPCAddr, "gRPC listen address.")
	fs.StringVar(&o.HTTPAddr, prefix+".http-addr", o.HTTPAddr, "HTTP listen address.")
	fs.StringVar(&o.Host, prefix+".host", o.Host, "Advertised host for registration; empty auto-detects (POD_IP, then the outbound interface).")
	fs.IntVar(&o.Port, prefix+".port", o.Port, "Advertised port for registration; 0 registers the port actually listened on.")
	fs.StringVar(&o.Env, prefix+".env", o.Env, "Deployment environment (dev/test/stg/prod), published as instance metadata.")
	fs.StringVar(&o.Version, prefix+".version", o.Version, "Service version, published as instance metadata.")
	fs.StringToStringVar(&o.Metadata, prefix+".metadata", o.Metadata, "Arbitrary extra instance metadata as key=value, repeatable.")
	fs.StringSliceVar(&o.MiddlewareRoutes, prefix+".middleware-route", o.MiddlewareRoutes, "Route-level middleware bindings, e.g. /svc.v1.Admin/*=ratelimit.")
}

// AdvertiseHost returns the address this instance is registered under.
//
// A listen address is not a registration address. A service listening on
// 0.0.0.0:8180 accepts on every interface, but a peer dialing 0.0.0.0 in a
// registry entry reaches itself — so passing the listen address straight
// through, which is what this used to do, publishes an instance nobody can
// call. The two addresses agree on a developer's machine and diverge everywhere
// else, which is exactly why the mistake survives local testing.
//
// The order is: an explicit host, then POD_IP, then the interface the kernel
// would use to reach the network. POD_IP is deliberately ahead of probing: a
// container often holds more than one interface, and reading the address the
// kubelet already wrote down beats guessing.
func (o *MeshOptions) AdvertiseHost() (string, error) {
	if h := strings.TrimSpace(o.Host); h != "" && !isUnspecifiedHost(h) {
		return h, nil
	}
	if ip := strings.TrimSpace(os.Getenv("POD_IP")); ip != "" {
		return ip, nil
	}
	return localIP()
}

// AdvertiseAddr splits a listen address and returns the host and port this
// instance should be registered under.
//
// It is the one place that decides what a peer is told, so the registrar and
// the instance description cannot disagree — and so a backend that carries the
// endpoint in the instance (etcd, kubernetes) is fixed by the same change as
// one that carries it in Options (polaris, consul).
//
// The port comes from the listen address unless Port overrides it: the port
// actually bound is the one a peer can reach, and a second default here would
// be a second source of truth for the same number.
func (o *MeshOptions) AdvertiseAddr(listenAddr string) (string, int, error) {
	_, portStr, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", 0, fmt.Errorf("parse listen address %q: %w", listenAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q", portStr)
	}
	if o.Port != 0 {
		port = o.Port
	}
	host, err := o.AdvertiseHost()
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

// isUnspecifiedHost reports whether host is a wildcard bind address ("0.0.0.0"
// or "::"), which is meaningful to a listener and useless to a peer.
func isUnspecifiedHost(host string) bool {
	if host == "0.0.0.0" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsUnspecified()
	}
	return false
}

// localIP returns the local address the kernel would use to reach a
// non-loopback destination.
//
// The UDP "dial" sends nothing — it only makes the kernel choose a source
// address from the routing table, which is the interface a peer would see. It
// therefore works with no network reachable, and picks the routable interface
// rather than, say, a docker bridge.
func localIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("determine local IP: %w", err)
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", fmt.Errorf("determine local IP: unexpected address type %T", conn.LocalAddr())
	}
	return addr.IP.String(), nil
}
