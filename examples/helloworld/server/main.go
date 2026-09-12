// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// helloworld-server demonstrates a dual-protocol (gRPC + gin HTTP) service
// assembled entirely from ServerOptions via the app composition root
// (app.RunMesh). Flags like --mesh.service-name, --mesh.protocol, and
// --registry.type replace the previously hand-wired bootstrap.
package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	proto "github.com/onexstack/onexmesh/examples/helloworld/proto"
	"github.com/onexstack/onexmesh/pkg/app"
	"github.com/onexstack/onexmesh/pkg/codec"
	"github.com/onexstack/onexmesh/pkg/options"
)

type greeterServer struct {
	proto.UnimplementedGreeterServer
}

func (s *greeterServer) SayHello(ctx context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
	return &proto.HelloReply{Message: "Hello " + req.GetName()}, nil
}

func main() {
	opts := options.NewServerOptions()
	// Sensible defaults for the demo; each can be overridden by flags.
	opts.Mesh.ServiceName = "edu.course.student-api"
	opts.Mesh.Protocol = "both"
	opts.Mesh.GRPCAddr = "127.0.0.1:9090"
	opts.Mesh.HTTPAddr = "127.0.0.1:8080"
	opts.Registry.Type = "etcd"

	engine := gin.New()
	engine.GET("/hello", func(c *gin.Context) {
		codec.Render(c, http.StatusOK, &proto.HelloReply{Message: "Hello " + c.Query("name")})
	})
	// Protobuf-in, protobuf-out HTTP endpoint demonstrating codec.Bind/Render.
	engine.POST("/hello", func(c *gin.Context) {
		var req proto.HelloRequest
		if err := codec.Bind(c, &req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		codec.Render(c, http.StatusOK, &proto.HelloReply{Message: "Hello " + req.GetName()})
	})

	a := app.NewApp("helloworld-server", "Helloworld gRPC + HTTP server.",
		app.WithOptions(opts),
		app.WithRun(app.RunMesh(opts,
			func(s grpc.ServiceRegistrar) { proto.RegisterGreeterServer(s, &greeterServer{}) },
			engine,
		)),
	)
	a.Run()
}
