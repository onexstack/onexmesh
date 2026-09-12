# OneXMesh

OneXMesh 是一个高性能、可扩展的 Go 微服务框架，统一了 gRPC 与 HTTP 双协议、可插拔的服务注册发现、配置中心与标准 OpenTelemetry 可观测体系。

## 特性

- **统一传输抽象**：`transport.Transporter` 抹平 HTTP header 与 gRPC metadata，一套中间件同时服务两种协议。
- **双协议**：gRPC（grpc + protobuf）与 HTTP（gin + JSON / gin + Protobuf）。
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
├── server/        # 生命周期（Server/ServiceGroup + http/grpc）
├── client/        # 服务发现客户端（Dial + onexmesh resolver + HTTP 实例缓存）
├── resilience/    # 韧性（breaker/shedder/retry/hedge/timeout）
├── ratelimit/     # 限流（本地 + 分布式 Store：memory/redis，含兜底）
├── event/         # 进程内事件总线 + 有界队列
├── core/          # 基础组件（collection/syncx/mr/contextx/timex/limit/stat）
├── errno/         # 框架级哨兵错误（*errorsx.ErrorX）
├── options/       # IOptions 配置 + BuildXxx 装配
├── app/           # CLI bootstrap + 组合根 RunMesh
└── version/       # 版本信息
```

## 快速开始

### 服务端（gRPC + HTTP 双协议，注册到 Etcd）

**组合根（推荐）**：`app.RunMesh` 作为唯一装配点，把 `ServerOptions` 配置转为运行中的服务，
自动完成 registrar 创建、中间件装配、启动与优雅关停：

```go
opts := options.NewServerOptions()
opts.Mesh.ServiceName = "edu.course.student-api"
opts.Mesh.Protocol = "both"
opts.Mesh.GRPCAddr = "127.0.0.1:9090"
opts.Mesh.HTTPAddr = "127.0.0.1:8080"
opts.Registry.Type = "etcd"

engine := gin.New()
engine.GET("/hello", func(c *gin.Context) { c.String(http.StatusOK, "Hello %s", c.Query("name")) })

a := app.NewApp("helloworld-server", "Helloworld gRPC + HTTP server.",
    app.WithOptions(opts),
    app.WithRun(app.RunMesh(opts,
        func(s grpc.ServiceRegistrar) { proto.RegisterGreeterServer(s, &greeterServer{}) },
        engine,
    )),
)
a.Run()
```

**底层 API（进阶）**：直接使用 `server`/`registry` 抽象手工装配，适合需要精细控制的场景：

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

### 客户端（按服务名发现）

```go
conn, _ := client.Dial(ctx, "edu.course.student-api",
    client.WithRegistry("etcd", etcd.Options{Endpoints: []string{"127.0.0.1:2379"}}))
grpcClient := proto.NewGreeterClient(conn)
resp, _ := grpcClient.SayHello(ctx, &proto.HelloRequest{Name: "world"})
```

完整示例见 `examples/helloworld/`。

## 服务发现 SDK 生成（fz-clientgen）

在 fz-clientgen 仓库中，通过 `onexmesh.v1.mesh_service` option 开启服务发现客户端生成：

```proto
import "proto/onexmesh/v1/onexmesh.proto";

option (onexmesh.v1.mesh_service) = {
  enable_service_discovery: true
  service_name: "edu.course.student-api"
  registry: "polaris"
};
```

生成的 SDK 默认携带服务名，通过 onexmesh 运行时完成服务发现：

```go
client, _ := v1.NewDeploymentServiceMeshClient(ctx) // 服务名已固化
resp, _ := client.GetDeployment(ctx, &v1.GetDeploymentRequest{Name: "d1"})
```
