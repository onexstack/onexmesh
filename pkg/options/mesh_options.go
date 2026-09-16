// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"

	"github.com/spf13/pflag"
)

var _ IOptions = (*MeshOptions)(nil)

// MeshOptions holds the core service identity and listen addresses.
type MeshOptions struct {
	ServiceName string `mapstructure:"service-name"`
	Protocol    string `mapstructure:"protocol"` // "grpc", "http", or "both"
	GRPCAddr    string `mapstructure:"grpc-addr"`
	HTTPAddr    string `mapstructure:"http-addr"`
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	// MiddlewareRoutes are optional route-level middleware bindings, each of
	// the form "selector=mw1,mw2" (e.g. "/svc.v1.Admin/*=ratelimit").
	MiddlewareRoutes []string `mapstructure:"middleware-route"`
}

// NewMeshOptions returns default mesh options.
func NewMeshOptions() *MeshOptions {
	return &MeshOptions{
		Protocol: "grpc",
		GRPCAddr: "0.0.0.0:9090",
		HTTPAddr: "0.0.0.0:8080",
		Host:     "127.0.0.1",
		Port:     9090,
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
	return errs
}

func (o *MeshOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&o.ServiceName, prefix+".service-name", o.ServiceName, "Service name, e.g. edu.course.student-api.")
	fs.StringVar(&o.Protocol, prefix+".protocol", o.Protocol, "Protocol: grpc, http, or both.")
	fs.StringVar(&o.GRPCAddr, prefix+".grpc-addr", o.GRPCAddr, "gRPC listen address.")
	fs.StringVar(&o.HTTPAddr, prefix+".http-addr", o.HTTPAddr, "HTTP listen address.")
	fs.StringVar(&o.Host, prefix+".host", o.Host, "Local host for registration.")
	fs.IntVar(&o.Port, prefix+".port", o.Port, "Local port for registration.")
	fs.StringSliceVar(&o.MiddlewareRoutes, prefix+".middleware-route", o.MiddlewareRoutes, "Route-level middleware bindings, e.g. /svc.v1.Admin/*=ratelimit.")
}
