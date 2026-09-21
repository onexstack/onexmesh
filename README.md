# OneXMesh

OneXMesh 是一个高性能、可扩展的 Go 微服务框架，统一了 gRPC 与 HTTP 双协议、可插拔的服务注册发现、配置中心与标准 OpenTelemetry 可观测体系。

## 特性

- **统一传输抽象**：`transport.Transporter` 抹平 HTTP header 与 gRPC metadata，一套中间件同时服务两种协议。
- **统一业务处理器**：`middleware.Handler`（签名与 `grpc.UnaryHandler` 一致）作为唯一业务契约，经 `GinHandler`/`UnaryServerInterceptor`/`StreamServerInterceptor` 桥，一份业务函数即可同时服务 gRPC 与 HTTP。
- **协议无关服务声明**：`server.NewMethod[Req, Resp]` 泛型把强类型业务函数擦除为统一 `middleware.Handler`，`server.Service` 一份声明同时驱动 gRPC（`Register`）与 HTTP 路由（`Methods`）。
- **protobuf IDL 驱动的 HTTP 路由**：方法级 `onexmesh.v1.http` 注解 + `protoc-gen-onexmesh` 插件生成 HTTP 路由（path/query/body 解码），与 gRPC 共用同一业务实现（grpc-gateway 风格）。
- **HTTP 路由插件化 + 流式注册**：`server.HTTPRoute` 自描述接口 + `RegisterHTTPRoute` 注册表，业务包 `init()` 以 Gin 风格的 `server.RouteGroup`（`NewGroup/Group/Use/GET/POST...`）自注册（支持前缀/分组/嵌套/组中间件），组合根 `server.NewMeshServer(...WithRoute(server.AllHTTPRoutes()...))` 零配置发现装配（与 registry/middleware/codec/selector 注册表同构）。
- **双协议**：gRPC（grpc + protobuf）与 HTTP（gin + JSON / gin + Protobuf），可按需开启 `grpc | http | both`。
- **可插拔注册中心**：工厂注册表 + `Registrar`/`Discovery` 分离，内置 Polaris、Etcd、Kubernetes、Consul、Nacos、Eureka。
- **服务发现缓存层**：`registry/cache` 为 `Discovery` 提供 singleflight 去重、服务/节点双 TTL、故障降级（stale-while-error）与刷新限流。
- **可插拔配置中心**：Source→Loader→Reader→Value 四层抽象，内置本地文件与 Polaris 配置中心；Source 通过注册表按名创建。
- **服务发现 SDK**：`client.Dial(ctx, "edu.course.student-api")` 通过 `onexmesh://` resolver 按服务名发现并调用。
- **负载均衡**：策略化 selector（round_robin / random / weighted / p2c），并接入 gRPC balancer。
- **韧性**：滑动窗口熔断、自适应降载、重试、对冲（backup request）、超时，作为可组合的 middleware，已接入 gRPC 与 HTTP 客户端。
- **限流**：本地令牌桶/漏桶/并发限流 + 分布式 `ratelimit.Store`（内存/Redis 双实现），带本地兜底与三态窗口限流。
- **统一错误体系**：复用 `onexstack/pkg/errorsx` 的 `ErrorX{Code/Reason/Message/Metadata}`，`pkg/errno` 定义框架级哨兵错误，`GRPCStatus()` 与 gin 桥保证 HTTP/gRPC 错误语义一致。
- **路由级中间件**：`middleware/matcher` 按 operation（gRPC 方法或 `METHOD /path`）为不同接口挂不同中间件。
- **可观测**：标准 OpenTelemetry trace/metric/log（otel/file/console/classic/hybrid 五态）+ log/slog（TraceIDHandler 关联 trace）。
- **事件总线**：`event` 进程内发布订阅 + 有界队列，用于解耦（如发现变更联动熔断清理）。
- **core 基础组件**：`collection`（RollingWindow/TimingWheel/SafeMap/Set）、`syncx`（SingleFlight/SpinLock）、`mr`（泛型 MapReduce）、`contextx`（ValueOnlyFrom）、`timex`（Ticker/FakeTicker）。
- **CLI 工具**：`onexmeshctl` 提供服务治理、配置读取、环境自检。

## 目录结构

```
pkg/
├── transport/     # 统一传输抽象（Transporter/Header）
├── middleware/    # 统一中间件（gin/grpc/stream 桥）+ matcher（路由级）+ suite + 内置 auth/cors/bodylimit/ratelimit
├── registry/      # 注册发现抽象 + polaris/etcd/kubernetes/consul/nacos/eureka 实现 + cache 缓存层
├── selector/      # 负载均衡（round_robin/random/weighted/p2c）
├── config/        # 配置四层抽象 + file/polaris source + 注册表
├── server/        # 组合根（MeshServer）+ 生命周期（Server/ServiceGroup + http/grpc）+ Service/Method 声明 + HTTPRoute 插件注册表
├── client/        # 服务发现客户端（Dial + onexmesh resolver + HTTP 实例缓存）
├── resilience/    # 韧性（breaker/shedder/retry/hedge/timeout）
├── ratelimit/     # 限流（本地 + 分布式 Store：memory/redis，含兜底）
├── event/         # 进程内事件总线 + 有界队列
├── core/          # 基础组件（collection/syncx/mr/contextx/timex/limit/stat）
├── errno/         # 框架级哨兵错误（*errorsx.ErrorX）
├── options/       # IOptions 配置 + ServerOptions 根
└── version/       # 版本信息
```

CLI bootstrap（cobra + viper + 信号 + 优雅关停）由外部 `github.com/onexstack/onexstack/pkg/app` 提供，
`MeshServer.Run` 满足其 `app.RunFunc` 契约，经 `app.WithRun(mesh.Run)` 接入。

## 快速开始

### 服务端（gRPC + HTTP 双协议，注册到 Etcd）

**组合根（推荐）**：`server.NewMeshServer` 作为唯一装配点，`opts.Mesh.Protocol = grpc|http|both`
按需开启协议，自动完成 registrar 创建、中间件装配、启动与优雅关停。业务逻辑只写一份强类型函数，
由 `protoc-gen-onexmesh` 生成的 `New<Service>Service` 打包成一份 `server.Service` 同时驱动 gRPC 注册
与 HTTP 路由：

```go
opts := options.NewServerOptions()
opts.Mesh.ServiceName = "edu.course.student-api"
opts.Mesh.Protocol = "both"
opts.Mesh.GRPCAddr = "127.0.0.1:9090"
opts.Mesh.HTTPAddr = "127.0.0.1:8080"
opts.Registry.Type = "etcd"

// proto.NewGreeterService 由 protoc-gen-onexmesh 生成：把 gRPC 注册（RegisterGreeterServer）
// 与 HTTP 路由打包成一个 server.Service，业务方无需写 grpc.ServiceRegistrar / 路由装配样板。
svc := proto.NewGreeterService(srv)

mesh := server.NewMeshServer(opts, server.WithService(svc))

a := app.NewApp("helloworld-server", "Helloworld gRPC + HTTP server.",
    app.WithOptions(opts),
    app.WithRun(mesh.Run),
)
a.Run()
```

`NewGreeterService` 内部使用 `server.NewMethod[Req, Resp]` 泛型擦除：每个方法的 `Handler` 就是
`GreeterServer` 接口的同名方法，因此一个 `srv.SayHello` 同时服务 gRPC 与 HTTP。

### protobuf IDL 驱动的 HTTP 路由（grpc-gateway 风格）

请求参数统一用 protobuf IDL 定义，HTTP 路由由方法级 `onexmesh.v1.http` 注解指定，
经 `protoc-gen-onexmesh` 插件生成，与 gRPC 共用同一份业务实现：

```proto
// hello.proto
import "onexmesh/v1/http.proto";

service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply) {
    option (onexmesh.v1.http) = { get: "/helloworld/{name}" };   // path 参数
  }
  rpc SayHelloPost(HelloRequest) returns (HelloReply) {
    option (onexmesh.v1.http) = { post: "/helloworld", body: "*" };  // body 参数
  }
}
```

```go
// 一个 struct 同时实现 gRPC 接口与生成的 HTTP 路由。
type greeterService struct{ proto.UnimplementedGreeterServer }
func (s *greeterService) SayHello(ctx context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
    return &proto.HelloReply{Message: "Hello " + req.GetName()}, nil
}
func (s *greeterService) SayHelloPost(ctx context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) { ... }

// 一行装配：NewGreeterService 把 gRPC + HTTP 注册打包成一个 server.Service。
srv := &greeterService{}
svc := proto.NewGreeterService(srv)
mesh := server.NewMeshServer(opts, server.WithService(svc))
```

生成命令：`make proto`（会 `go install` 两个插件并重新生成）。
`protoc-gen-onexmesh` 会额外生成 `New<Service>Service(srv)` 工厂，把 `Register<Service>Server`
（gRPC）与 proto-first HTTP 路由两个固定样板收进生成代码，业务方无需写
`grpc.ServiceRegistrar` / 路由装配样板。

> 完整示例见 `examples/helloworld/`（含自定义中间件 `requestID`、原生 gin 端点 `healthz`、
> 手写 path/query 扩展 `/users/:id` 与 `/search`）。`opts.Mesh.Protocol = grpc|http|both` 按需开启。
> gRPC streaming 经 `Service.Register` 扩展点接入。

**程序化声明（进阶，无 proto 生成）**：直接使用 `server.NewService` + `server.NewMethod` 手工装配，
适合需要精细控制或不走 IDL 生成的场景：

```go
svc := server.NewService("helloworld.Greeter",
    server.NewMethod("SayHello", "POST", "/hello", "*",
        func(ctx context.Context, r *proto.HelloRequest) (*proto.HelloReply, error) {
            return &proto.HelloReply{Message: "Hello " + r.GetName()}, nil
        },
    ),
)
mesh := server.NewMeshServer(opts, server.WithService(svc))
```

> 注意：`NewService` 只声明 HTTP 路由；若还要服务 gRPC，需额外设置 `Service.Register`
> （生成的 `Register<Service>Server`）。推荐直接使用生成的 `New<Service>Service`，它两者都带。

**底层 API（进阶）**：直接使用 `server`/`registry` 抽象手工装配：

```go
registrar, _ := registry.CreateRegistrar("etcd", etcd.Options{Endpoints: []string{"127.0.0.1:2379"}, TTL: 15})

grpcSrv := server.NewGRPCServer("127.0.0.1:9090", func(s grpc.ServiceRegistrar) {
    proto.RegisterGreeterServer(s, &greeterServer{})
}).WithRegistrar(registrar, &registry.ServiceInstance{
    Name:      "edu.course.student-api",
    Endpoints: []string{"grpc://127.0.0.1:9090", "http://127.0.0.1:8080"},
})

httpSrv := server.NewHTTPServer("127.0.0.1:8080", engine,
    server.WithRegistrar(registrar, &registry.ServiceInstance{
        Name:      "edu.course.student-api",
        Endpoints: []string{"grpc://127.0.0.1:9090", "http://127.0.0.1:8080"},
    }))

group := server.NewServiceGroup()
group.Add("grpc", grpcSrv)
group.Add("http", httpSrv)
_ = group.Start(ctx)
```

### 原生 Gin 路由（直接基于 *gin.Engine）

少量纯 HTTP 端点（path/query 参数、无 proto 定义）可直接拿到带统一中间件链的原生 `*gin.Engine`，
以 Gin 风格注册，无需 `server.NewGroup` / `RegisterHTTPRoute` 与路由名；若同时有 gRPC service，
用 `server.NewMeshServer(...WithGinEngine(engine), WithService(svc))` 装配：

```go
opts := options.NewServerOptions()
opts.Mesh.Protocol = "both"
opts.Mesh.GRPCAddr = "127.0.0.1:9090"
opts.Mesh.HTTPAddr = "127.0.0.1:8080"
opts.Registry.Type = "etcd"

// server.NewGinEngine 返回注入统一中间件链的原生 *gin.Engine。
engine, _ := server.NewGinEngine(opts)
engine.GET("/healthz", func(c *gin.Context) {
    codec.Render(c, http.StatusOK, map[string]string{"status": "ok"})
})

// 原生 gin 分组：组级统一中间件经 middleware.GinHandler 桥接。
v1 := engine.Group("/v1", middleware.GinHandler(apiVersion("v1")))
v1.GET("/posts/:postID", handler.GetPost)
v1.POST("/posts", handler.CreatePost)

svc := proto.NewGreeterService(srv) // 可选：与 gRPC/proto-first 路由共存

mesh := server.NewMeshServer(opts, server.WithGinEngine(engine), server.WithService(svc))
a := app.NewApp("server", "my service", app.WithOptions(opts), app.WithRun(mesh.Run))
a.Run()
```

纯 HTTP（无 gRPC）时 `opts.Mesh.Protocol = "http"`，同样用 `WithGinEngine` 装配即可。
该方式适合少量路由直接注册；多业务包插件式自注册场景见下一节 `RegisterHTTPRoute`。

### HTTP 路由插件化注册（流式 RouteGroup）

独立于 `server.Service` 的 HTTP-only 路由模块可通过 `server.RegisterHTTPRoute` 在 `init()`
自注册，组合根 `server.NewMeshServer(...WithRoute(server.AllHTTPRoutes()...))` 自动发现装配，
新增路由模块无需改组合根。路由以 `server.RouteGroup` 流式声明（Gin `RouterGroup` 风格：
`NewGroup / Group / Use / GET / POST / ...`），但记录的是数据而非命令式改引擎，天然支持前缀、
分组、嵌套与组中间件：

```go
// 业务包内（可独立成包）：init 自注册，无需组合根显式 import。
func init() {
    // 原生 gin 逃逸口（path/query 参数）：GET /users/:id
    users := server.NewGroup("")
    users.GET("/users/:id", func(c *gin.Context) {
        codec.Render(c, http.StatusOK, userReply{ID: c.Param("id"), Name: users[c.Param("id")]})
    })
    server.RegisterHTTPRoute("users", users)

    // 前缀 / 分组 / 嵌套 / 组中间件
    api := server.NewGroup("/api/v1", apiVersion("v1"))
    api.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
    admin := api.Group("/admin")
    admin.GET("/stats", handler)
    server.RegisterHTTPRoute("api", api)
}

// 组合根：自动发现所有已注册 HTTPRoute 插件（可同时传入显式 server.Service）。
mesh := server.NewMeshServer(opts, server.WithRoute(server.AllHTTPRoutes()...))
a := app.NewApp("server", "my service", app.WithOptions(opts), app.WithRun(mesh.Run))
a.Run()
```

`server.RouteGroup` 同时满足 `server.HTTPRoute` 接口，故可直接 `RegisterHTTPRoute(name, group)`；
多条路由在同一组上链式 `.GET(...).POST(...)`，嵌套用 `.Group(prefix, mws...)`，组中间件用
`NewGroup(prefix, mws...)` 或 `.Use(mws...)`。原生 gin handler 走 `.GET/.POST/.../.Handle`。
该模式与 `registry.RegisterBackend` / `middleware.Register` / `codec.Register` /
`selector.RegisterSelector` 同构（注册表 + `init()` 自注册 + 按名发现，符合开放/封闭原则）。

### 客户端（按服务名发现）

```go
conn, _ := client.Dial(ctx, "edu.course.student-api",
    client.WithRegistry("etcd", etcd.Options{Endpoints: []string{"127.0.0.1:2379"}}))
grpcClient := proto.NewGreeterClient(conn)
resp, _ := grpcClient.SayHello(ctx, &proto.HelloRequest{Name: "world"})
```

完整示例见 `examples/helloworld/`。

## SDK 生成（protoc-gen-onexmesh-client）

onexmesh 内置 `protoc-gen-onexmesh-client` 插件，从**资源型 protobuf IDL** 生成 client-go 风格的
REST SDK（clientset / typed / fake / informer / lister / applyconfigurations），并内建双协议服务治理。
它与服务端的 `protoc-gen-onexmesh`（HTTP 路由）互补：前者面向 resource + verbs，后者面向
service + method，二者互不重叠。

### 资源与聚合锚点

```proto
// examples/apis/apps/v1/deployment.proto
option (onexmesh.rest.v1.file) = { group: "apps" version: "v1" };

message Deployment {
  option (onexmesh.rest.v1.resource) = {};
  string apiVersion = 1;
  string kind = 2;
  onexmesh.meta.v1.ObjectMeta metadata = 3;
  DeploymentSpec spec = 4;
}

// examples/pkg/generated/exampleclient/clientset.proto（聚合锚点）
option (onexmesh.rest.v1.clientset) = {
  name: "exampleclient"
  groups: [ { group: "apps" version: "v1" } ]
};
```

### 调用方式

```go
// 纯 client-go 风格（fake 内存测试）
client := fake.NewSimpleClientset(deployment)
client.AppsV1().Deployments("default").Get(ctx, "d1", metav1.GetOptions{})

// REST mesh-aware：NewForMesh 通过注册中心发现服务并负载均衡
client, _ := exampleclient.NewForMesh("edu.course.student-api",
    rest.WithRegistry("etcd", &etcd.Options{Endpoints: []string{"127.0.0.1:2379"}}))
client.AppsV1().Deployments("default").List(ctx, metav1.ListOptions{})

// gRPC mesh client：mesh_service option 固化服务名，发现由 onexmesh 运行时完成
option (onexmesh.v1.mesh_service) = {
  service_name: "edu.course.student-api"
  registry: "etcd"
};
grpcClient, _ := appsv1.NewDeploymentServiceMeshClient(ctx)

// 类型化 HTTP 客户端：同一份 mesh_service 标注 + 方法级 onexmesh.v1.http 注解，
// 同时生成类型化 HTTP 客户端（服务发现 + path/body 绑定，用法与 gRPC 一致）
httpClient, _ := appsv1.NewDeploymentServiceHTTPClient(ctx, client.WithHTTPRegistry("etcd", &etcd.Options{}))
reply, _ := httpClient.Get(ctx, &appsv1.GetRequest{Name: "d1"})
```

完整示例见 `examples/apis/apps/v1/`（资源 + gRPC service）与
`examples/pkg/generated/exampleclient/`（生成的 SDK 树）。
