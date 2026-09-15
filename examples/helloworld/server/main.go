// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// helloworld-server 演示 OneXMesh 的「protobuf IDL 驱动的 gRPC + Gin 统一」：
//
//   - 单一 IDL：examples/helloworld/proto/hello.proto 定义 service 方法、请求/响应 message，
//     以及 onexmesh.v1.http 注解（HTTP 路由）。
//   - 单一实现：greeterService 一个 struct 同时实现 GreeterServer（gRPC）与生成的 HTTP 路由，
//     SayHello 对应 GET /helloworld/{name}（path 参数），SayHelloPost 对应 POST /helloworld（body）。
//   - 一行装配：proto.NewGreeterService(srv) 把 gRPC 与 HTTP 注册打包成一个 server.Service，
//     业务方无需写 grpc.ServiceRegistrar / 路由装配样板。
//   - 纯 HTTP 端点（无 proto 定义）：healthz / users / search / api 直接基于 server.NewGinEngine 返回的
//     原生 *gin.Engine 以 Gin 风格注册（Group/GET/...，支持前缀 / 分组 / 嵌套 / 组中间件）。
//   - 按需开启：opts.Mesh.Protocol = grpc|http|both 控制只起 gRPC / 只起 Gin / 双起。
//   - 灵活定制：requestID 自定义中间件（双协议）。
package main

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	proto "github.com/onexstack/onexmesh/examples/helloworld/proto"
	"github.com/onexstack/onexmesh/pkg/codec"
	"github.com/onexstack/onexmesh/pkg/middleware"
	"github.com/onexstack/onexmesh/pkg/options"
	_ "github.com/onexstack/onexmesh/pkg/registry/all"
	"github.com/onexstack/onexmesh/pkg/server"
	"github.com/onexstack/onexmesh/pkg/transport"
	app "github.com/onexstack/onexstack/pkg/app"
	"github.com/onexstack/onexstack/pkg/errorsx"
)

// requestID 是一个自定义的协议无关中间件：为每个请求注入 request id、写回响应头，
// 并记录带 operation/kind/耗时的结构化日志。它通过 transport.Transporter 读取协议
// 元数据，因此同一份实现同时服务 gRPC 与 HTTP。
func requestID() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			id := strconv.FormatInt(time.Now().UnixNano(), 10)
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return next(ctx, req)
			}
			tr.ReplyHeader().Set("X-Request-Id", id)

			start := time.Now()
			resp, err := next(ctx, req)

			slog.Info("request handled",
				"request_id", id,
				"kind", tr.Kind(),
				"operation", tr.Operation(),
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return resp, err
		}
	}
}

// apiVersion 是演示分组级中间件：给组内所有路由写一个版本响应头，说明统一中间件
// 可经 middleware.GinHandler 桥接为 gin 的 Group 中间件，只对组内路由生效。
func apiVersion(v string) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				tr.ReplyHeader().Set("X-API-Version", v)
			}
			return next(ctx, req)
		}
	}
}

// userReply / searchReply 是 HTTP 特有响应的演示类型（path/query 参数无 gRPC 对应）。
type userReply struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type searchReply struct {
	Query string   `json:"query"`
	Limit int      `json:"limit"`
	Items []string `json:"items"`
}

// users 是 path 参数演示的内存数据。
var users = map[string]string{
	"1": "alice",
	"2": "bob",
}

func init() {
	// 注册到名字表，供 Mesh.MiddlewareRoutes 按名引用（"*=requestid" 表示对所有 operation 生效）。
	middleware.Register("requestid", func() middleware.Middleware { return requestID() })
}

// greeterService 是唯一业务实现：一个 struct 同时满足 gRPC 的 GreeterServer 接口与
// 生成的 HTTP 路由（GreeterRouteGroups），实现 gRPC/HTTP 代码处理一致。
type greeterService struct {
	proto.UnimplementedGreeterServer
}

// SayHello 对应 GET /helloworld/{name}（path 参数由 onexmesh.v1.http 注解指定）。
func (s *greeterService) SayHello(ctx context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
	return &proto.HelloReply{Message: "Hello " + req.GetName()}, nil
}

// SayHelloPost 对应 POST /helloworld（body:"*" 由注解指定，整 message 作为请求体）。
func (s *greeterService) SayHelloPost(ctx context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
	return &proto.HelloReply{Message: "Hello (post) " + req.GetName()}, nil
}

func main() {
	opts := options.NewServerOptions()
	// Sensible defaults for the demo; each can be overridden by flags.
	opts.Mesh.ServiceName = "edu.course.student-api"
	opts.Mesh.Protocol = "both" // grpc | http | both：按需开启
	opts.Mesh.GRPCAddr = "127.0.0.1:9090"
	opts.Mesh.HTTPAddr = "127.0.0.1:8080"
	opts.Registry.Type = "etcd"
	// 通过路由级中间件注入自定义 requestid（"*" 匹配所有 gRPC 方法 / HTTP 路由）。
	opts.Mesh.MiddlewareRoutes = []string{"*=requestid"}

	srv := &greeterService{}

	// 原生 gin：拿到带统一中间件链的 *gin.Engine，直接以 Gin 风格注册纯 HTTP 端点，
	// 无需 server.NewGroup / RegisterHTTPRoute 与路由名。
	engine, err := server.NewGinEngine(opts)
	if err != nil {
		panic(err)
	}

	// 无请求体的纯 HTTP 端点（无需 proto 定义）。
	engine.GET("/healthz", func(c *gin.Context) {
		codec.Render(c, http.StatusOK, map[string]string{"status": "ok"})
	})

	// path 参数演示：GET /users/:id。
	engine.GET("/users/:id", func(c *gin.Context) {
		id := c.Param("id")
		name, ok := users[id]
		if !ok {
			codec.RenderError(c, errorsx.ErrNotFound.WithMessage("user %s not found", id))
			return
		}
		codec.Render(c, http.StatusOK, userReply{ID: id, Name: name})
	})

	// query 参数演示：GET /search?q=...&limit=...
	engine.GET("/search", func(c *gin.Context) {
		q := c.DefaultQuery("q", "")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		codec.Render(c, http.StatusOK, searchReply{Query: q, Limit: limit, Items: []string{"onex", "mesh"}})
	})

	// 分组/前缀/嵌套/组中间件演示：前缀 /api/v1 + 组中间件 apiVersion + 嵌套 /admin 子组。
	// 组级中间件（middleware.Middleware）经 middleware.GinHandler 桥接为 gin.HandlerFunc。
	api := engine.Group("/api/v1", middleware.GinHandler(apiVersion("v1")))
	api.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	api.Group("/admin").GET("/stats", func(c *gin.Context) {
		codec.Render(c, http.StatusOK, map[string]any{"requests": 42})
	})

	// 一行装配：NewGreeterService 把 gRPC 注册（RegisterGreeterServer）与生成的 HTTP 路由
	// （GreeterRouteGroups）打包成一个 server.Service，无需写 grpc.ServiceRegistrar / 路由装配样板。
	svc := proto.NewGreeterService(srv)

	// 用 onexstack 的 app 框架构建应用，用 server.NewMeshServer 装配 MeshServer；
	// mesh.Run 内部负责初始化 slog/otel、装配服务组、启动与优雅关停。
	mesh := server.NewMeshServer(opts, server.WithGinEngine(engine), server.WithService(svc))

	a := app.NewApp("helloworld-server", "Helloworld gRPC + HTTP server with IDL-driven routes.",
		app.WithOptions(opts),
		app.WithRun(mesh.Run),
	)
	a.Run()
}
