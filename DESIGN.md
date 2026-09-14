# OneXMesh 设计文档

本文档说明 OneXMesh 微服务框架的架构、设计模式落点与扩展方式。

## 1. 分层架构

```
pkg/
├── app/            # CLI 引导（cobra + viper + 信号 + 优雅关停钩子）+ 组合根 RunMesh
├── options/        # IOptions 配置（Validate + AddFlags），onex 约定 + BuildXxx 装配
├── server/         # 生命周期抽象（Server/GRPCServer/HTTPServer/ServiceGroup）
├── transport/      # 协议无关传输抽象（Transporter/Header）
├── middleware/     # 统一中间件（Handler/Middleware + gin/grpc/stream 桥）
│   └── matcher/    #   路由级中间件选择（精确/前缀/通配）
├── client/         # 服务发现客户端（Dial + onexmesh resolver + balancer + HTTP 实例缓存）
│   └── balancer/   # selector -> gRPC balancer 适配器
├── registry/       # 注册发现抽象（Registrar/Discovery/Watcher + 工厂）
│   ├── cache/      #   read-through 缓存（singleflight + 双 TTL + 降级）
│   ├── polaris/    #   Polaris 后端
│   ├── etcd/       #   etcd 后端
│   ├── kubernetes/ #   Kubernetes EndpointSlice 后端
│   ├── consul/     #   Consul 后端
│   ├── nacos/      #   Nacos 后端
│   └── eureka/     #   Eureka 后端（纯 REST）
├── selector/       # 负载均衡（策略 + DoneFunc 反馈）
│   ├── roundrobin/  random/  weighted/  p2c/
├── config/         # 配置四层抽象（Source->Loader->Reader->Value）
│   └── source/     #   file / polaris
├── codec/          # HTTP 消息编解码（json / protobuf）
├── resilience/     # 韧性原语（breaker / retry / hedge / shedder / timeout）
├── ratelimit/      # 限流（本地 + 分布式 Store）
│   └── store/      #   memory（默认）/ redis（Lua 脚本）
├── event/          # 进程内事件总线 + 有界队列
├── core/           # 基础组件（collection / syncx / mr / contextx / timex / limit / stat）
├── errno/          # 框架级哨兵错误（*errorsx.ErrorX）
├── otel/empty/     # 空 span 导出器（classic 模式）
├── otelslog/       # slog -> OTel log 桥
└── version/        # 版本信息
```

## 2. 设计模式落点

| 模式 | 应用环节 |
|---|---|
| 抽象工厂 + 注册表 | `registry.RegisterRegistrar/RegisterDiscovery` + `CreateXxx`；`selector.RegisterSelector/GetSelector`；`middleware.Register/Get`；`codec.Register/Get`；`config` 与 `ratelimit.Store` 同构 |
| 函数式 Option | `client.Dial(...WithXxx)`、`server.NewGRPCServer(...opts)`、`NewApp(...)` |
| 策略 | selector 负载均衡（round_robin/random/weighted/p2c）；codec（json/protobuf）；限流（令牌桶/漏桶/并发） |
| 适配器 | `transport.Transporter/Header` 抹平 gin/grpc；`client/balancer` 适配 selector→grpc；`middleware.GinHandler/UnaryServerInterceptor/StreamServerInterceptor/HTTPHandler` 桥 |
| 责任链 | middleware `Chain`；grpc `WithChainUnaryInterceptor`；resilience 链（`Chain` 统一收敛到 `core/chain` 泛型实现） |
| 装饰器 | middleware 包装 handler；`TraceIDHandler` 装饰 slog.Handler；`registry/cache` 装饰 `Discovery` |
| 观察者 | registry `Watcher.Next()`；config `Watch` 热更新 |
| 稳定性模式 | resilience（circuit/retrier/hedge/deadline/shedder）；p2c 的 EWMA 反馈闭环 |
| singleflight | `registry/cache` 并发查询去重（防击穿） |
| Builder | config Source 组装；OTel Provider 组装；`middleware.Builder` 运行时参数化 |
| 代理 | `onexmesh://` resolver + 服务发现客户端 |
| MapReduce | `core/mr` 泛型并发批处理 |

## 3. 核心契约

### 3.1 服务发现（registry）

`registry.ServiceInstance` 用 `Endpoints []string`（形如 `grpc://ip:port`）表达多协议端点；
`Registrar`（注册）与 `Discovery`（发现）接口分离（接口隔离原则）。

```go
type ServiceInstance struct { ID, Name, Version string; Metadata map[string]string; Endpoints []string }
type Registrar interface { Register(ctx, *ServiceInstance) error; Deregister(ctx, *ServiceInstance) error }
type Discovery interface { GetService(ctx, name) ([]*ServiceInstance, error); Watch(ctx, name) (Watcher, error) }
type Watcher interface { Next() ([]*ServiceInstance, error); Stop() error }
```

每个后端另有一个自描述入口 `registry.Backend`（`Name/AddFlags/NewRegistrar/NewDiscovery`），
经 `RegisterBackend` 在 `init()` 注册。`options.RegistryOptions` 只持有一个
`map[string]registry.Backend`，`AddFlags` 时遍历 `registry.BackendNames()` 让各后端贡献
`--registry.<name>.*` 嵌套 flags，`NewRegistrar/NewDiscovery` 按 `Type` 委托，不再有
god struct 或 6 分支 switch。`pkg/registry/all` 空导入全部内置后端，由组合根
`pkg/app` 引入以保证开箱即用。

### 3.2 服务发现客户端（client）

```go
conn, _ := client.Dial(ctx, "edu.course.student-api",
    client.WithRegistry("polaris", &polaris.Options{}),
    client.WithSelector("p2c"),
    client.WithDiscoveryCache(30*time.Second),              // 可选：read-through 缓存
    client.WithTimeout(5*time.Second),
    client.WithRetry(3, 100*time.Millisecond, time.Second),
    client.WithRetryable(func(err error) bool { /* 按错误分类决定是否重试 */ }),
    client.WithBreaker(10*time.Second, time.Second),        // 滑动窗口 + 探活间隔
)
```

`Dial` 通过 `onexmesh:///edu.course.student-api` resolver 按服务名发现，负载均衡由
`client/balancer`（selector→grpc balancer 适配器）执行，使框架 selector 策略真正生效。
`WithDiscoveryCache` 用 `registry/cache` 包装 `Discovery`（singleflight 去重 + 服务/节点双 TTL +
stale-while-error 降级 + 刷新限流）。HTTP 客户端 `NewHTTPClient` 提供对称的
`WithHTTPRegistry/WithHTTPSelector/WithHTTPRetry/WithHTTPBreaker/WithHTTPDiscoveryCache` 选项，
并内置本地实例缓存 + 后台 Watch 订阅。

### 3.3 中间件（middleware）

统一 `Handler func(ctx, req) (resp, error)` + `Middleware func(Handler) Handler`，
`UnaryServerInterceptor`/`UnaryClientInterceptor`/`StreamServerInterceptor`/`GinHandler`/`HTTPHandler`
五个桥适配 gRPC（unary + streaming）与 gin。其中 `HTTPHandler` 是业务层桥：它把一份 `Handler`
适配成 `gin.HandlerFunc`（`codec.Bind` 解码 body、`codec.Render` 编码响应、`codec.RenderError`
写错误包络），使一个业务函数同时服务 gRPC 方法与 HTTP 路由（见 `examples/helloworld`）。
unary 桥在 handler 返回后把 `ReplyHeader` 回写 `grpc.SetHeader`；streaming 桥由 `RunMesh` 经
`ChainStreamInterceptor` 接入。

内置中间件：`Recovery`、`Logging`、`Tracing`、`Metrics`（in-flight gauge + 错误分类计数）、
`Timeout`、`Auth`（JWT，复用 onexstack `pkg/token`）、`CORS`、`BodyLimit`、`RateLimit`。
`middleware.Register/Get` 提供名字注册表，`middleware/matcher` 提供路由级选择，
`middleware.Builder`/`Suite` 支持运行时参数化与套件组合。

### 3.4 配置（config）

`Source`（数据来源）→ `Loader`（解码+合并）→ `Reader`（随机访问）→ `Value`（类型访问），
source 支持 file 与 polaris 配置中心，可热更新。`Value` 返回深拷贝快照，避免与后台 Watch
合并产生数据竞争。

### 3.5 韧性（resilience）

所有韧性原语都是 `Middleware func(Handler) Handler`，可经 `resilience.Chain` 组合：

| 原语 | 说明 |
|---|---|
| `Breaker` | Google SRE 客户端限流滑动窗口熔断（`k/minK/protection/force-pass` 概率丢弃），`WithAcceptable` 错误分类（超时/4xx 不熔断），`NopBreaker` 桩 |
| `Shedder` | 自适应降载：CPU 阈值 + 利特尔法则 `maxFlight = maxPass×minRt×windowScale`，`Promise.Pass/Fail` 反馈 |
| `Retry` | 指数退避 + 抖动；`WithPerAttemptTimeout` 单次独立超时；`WithRetryable` 错误分类；`WithMaxRetryRatio` 重试流量占比限流（防重试风暴） |
| `Hedge` | backup request 对冲：主请求超延迟阈值发备份请求，取先返回者（降尾延迟） |
| `Deadline` | 截止期限：`min(parent deadline, now+d)` 预算传播，超时返回类型化 `errno.ErrTimeout`（仍可 `errors.Is` 到 `context.DeadlineExceeded`） |
| `Bulkhead` | 隔离舱：按下游限并发（`core/limit` 信号量），满则快速失败 `errno.ErrBulkheadFull`，阻断串联故障 |

客户端通过 `buildResilienceInterceptors`（gRPC）与 `buildResilienceHandler`（HTTP）把上述原语
按 deadline → bulkhead → breaker → retry 顺序接入，并用 `grpcAcceptable`/`grpcRetryable` 按 gRPC 状态码分类：
客户端错误（InvalidArgument/NotFound）不熔断、不重试；瞬态/服务端错误（Unavailable/Internal/
DeadlineExceeded 等）才触发。

`pkg/resilience` 是命令式原语层；`pkg/resiliency` 是叠加其上的声明式 Provider（Dapr 风格：
命名策略模板 + 目标绑定 + 状态码区间匹配 + 每端点熔断状态），经 `client.WithResiliency(p)`
接入。注意 `resiliency` 内的熔断是「连续失败阈值」语义，与 `resilience` 的「SRE 滑动窗口」
熔断是两种算法、非重复实现。

### 3.6 组合根（app.RunMesh）

`options.ServerOptions` 聚合各 leaf 配置，由 `app.RunMesh` 作为唯一装配点把「配置 → 依赖图」打通：

```go
opts := options.NewServerOptions()
opts.Mesh.ServiceName = "edu.course.student-api"
opts.Mesh.Protocol = "both"
opts.Registry.Type = "etcd"

a := app.NewApp("server", "my service",
    app.WithOptions(opts),
    app.WithRun(app.RunMesh(opts,
        func(s grpc.ServiceRegistrar) { proto.RegisterGreeterServer(s, &greeterServer{}) },
        engine,
    )),
)
a.Run()
```

`RunMesh` 内部：从 `RegistryOptions` 创建 registrar/discovery → 从 `MeshOptions` 构建
`ServiceInstance` → 从 `SelectorOptions`/`ResilienceOptions` 构建 client `DialOption` →
组装中间件链（全局或经 `BuildMatcher` 的路由级）→ 启动 `ServiceGroup`（gRPC+HTTP）→
上下文取消时优雅停服并 deregister。`options` 层的 `ServiceInstance/BuildMiddleware/BuildMatcher/
BuildClientDialOptions` 把「配置对象 → 运行时对象」转换集中在一处，保持无副作用、可测试。

`RunMeshWithServices` 是推荐的组合根：业务方用 `server.NewMethod[Req, Resp]` 泛型把一份强类型
函数擦除为统一 `middleware.Handler`，框架层用 gRPC 反射注册（`Service.RegisterGRPC` 运行时构造
`grpc.ServiceDesc`，无需生成的 `RegisterXxxServer`）与 HTTP 路由生成同时服务两种协议，再委托给
`RunMesh`。协议特有场景走扩展点：gRPC streaming 经 `Service.Register`、IDL 生成的 HTTP 路由经
`Service.RegisterHTTP`；纯 HTTP 端点（无 proto 定义）经 `server.RegisterHTTPRoute` 插件化。

### 3.7 protobuf IDL 驱动的 HTTP 路由（grpc-gateway 风格）

请求参数统一用 protobuf IDL 定义，HTTP 路由由方法级 `onexmesh.v1.http` 注解指定
（`pkg/proto/onexmesh/v1/http.proto` 的自包含 `HttpRule`，替代 `google.api.http`），
`cmd/protoc-gen-onexmesh` 插件读取注解并生成 `*_http.pb.go`：

- 生成 `Register<Service>HTTPServer(e *gin.Engine, srv <Service>Server)`，复用 gRPC 的
  `<Service>Server` 接口（单一接口、单一 struct 双协议）。
- 额外生成 `New<Service>Service(srv <Service>Server) server.Service` 工厂，把 gRPC 注册
  （`Register<Service>Server`）与 HTTP 注册（`Register<Service>HTTPServer`）两个固定样板收进
  生成代码，业务方一行 `proto.NewGreeterService(srv)` 完成装配，无需写 `grpc.ServiceRegistrar` /
  `*gin.Engine` 闭包。
- 每个 handler 依序 `codec.BindPath`（`{field}` path 参数）→ `codec.BindQuery`（query 参数）→
  `codec.Bind`（`body:"*"` 整 message），再调用同一业务方法、`codec.Render` 编码响应。
- `codec.BindPath/BindQuery` 用 protoreflect + strconv 做 string→标量/enum/repeated/嵌套 回填
  （对齐 grpc-gateway `runtime/query.go` 的子集，kratos 式可插拔解码）。

对应 grpc-gateway 的 `local_request_Xxx` 直调：HTTP 路由解码后直接调用 gRPC 服务实现的同一方法，
不做 client 转发。`opts.Mesh.Protocol = grpc|http|both` 控制按需开启。字段级 `body:"field"` 与
多 segment path `{name=messages/*}` 留作后续扩展。

`RunMeshRegistered` 是插件化入口：业务包在 `init()` 里 `server.RegisterHTTPRoute` 自注册
HTTP-only 路由模块（`server.HTTPRoute` 自描述接口），组合根自动发现装配，无需显式 import/汇总。
`RunMeshWithRoutes(opts, routes...)` 则按显式 `[]HTTPRoute` 装配纯 HTTP 服务。三者都委托 `RunMesh`，
共享中间件链、注册与生命周期。

## 4. 扩展指南

### 4.1 新增一个注册中心后端

后端通过 `registry.Backend` 接口自描述（自注册 flags、自构造 registrar/discovery），
`options` 包不再 import 或 switch 任何具体后端，符合开闭原则。

1. 新建 `pkg/registry/<name>/`，实现 `Options` 结构（可零值）、`registrar.go`、`discovery.go`。
2. 新建 `<name>/backend.go`，实现 `registry.Backend`（`Name/AddFlags/NewRegistrar/NewDiscovery`），
   并在 `init()` 里 `registry.RegisterBackend(name, newBackend)`。
3. 在 `pkg/registry/all/all.go` 中加一行 `_ "<name>"` 使其默认可用；
   `<name>.go` 的 `init()` 同时保留 `RegisterRegistrar/RegisterDiscovery` 以服务客户端
   ad-hoc 用法（`client.WithRegistry(name, &<name>.Options{})`）。
4. 无需再改 `options` 包；`--registry.<name>.*` 嵌套 flags 由后端 `AddFlags` 自动贡献。

参考实现：`pkg/registry/polaris/`（SDK）、`pkg/registry/consul/`（TTL 心跳）、
`pkg/registry/nacos/`（Subscribe 回调）、`pkg/registry/eureka/`（纯 REST）。

### 4.2 新增一个 selector 策略

1. 新建 `pkg/selector/<name>/`，实现 `selector.Selector` 接口
   （`Select(ctx, []Node) (Node, DoneFunc, error)`）。
2. `init()` 里 `selector.RegisterSelector("<name>", func() selector.Selector {...})`。
3. 使用 `client.WithSelector("<name>")`（已内置 round_robin/random/weighted/p2c）。

### 4.3 新增一个 config source

1. 实现 `config.Source`（`Load() / Watch()`）+ `config.Watcher`。
2. 通过 `config.New(config.WithSource(src))` 组装。

### 4.4 新增一个中间件

实现 `middleware.Middleware`（`func(Handler) Handler`），用 `middleware.Chain` 组合，
经 `middleware.GinHandler`/`UnaryServerInterceptor`/`StreamServerInterceptor` 桥接入
gin/grpc（含 streaming）。业务函数则经 `middleware.HTTPHandler` 桥实现「一份逻辑双协议」。
可选地 `middleware.Register(name, factory)` 注册到名字表，
供路由级配置按名引用。参考：`pkg/middleware/` 下的 `auth.go`（JWT）、`cors.go`、
`bodylimit.go`（`http.MaxBytesReader` 防 OOM）、`ratelimit.go`（`ratelimit.Limiter`）。

### 4.5 新增一个 codec

实现 `codec.Marshaler`（`ContentType/Marshal/Unmarshal`），`init()` 里 `codec.Register(name, m)`，
用 `codec.Get(name)` 解析。

### 4.6 新增一个限流 Store

实现 `ratelimit.Store`（`TakeTokens/IncrWindow/Ping/Close`）。`pkg/ratelimit/store/memory`
为零配置默认，`pkg/ratelimit/store/redis` 为分布式实现；`ratelimit.TokenLimiter` 在 Store
不可用时自动降级到进程内限流并后台探测恢复。

### 4.7 服务发现缓存与路由级中间件

- `registry/cache.New(discovery, cache.WithTTL(...))` 包装 `Discovery`，提供 singleflight、
  双 TTL、故障降级与刷新限流；客户端用 `client.WithDiscoveryCache(ttl)` 开启。
- `middleware/matcher` 通过 `Use`/`Add(selector)`/`Match(operation)` 做路由级中间件选择；
  `matcher.Match(m)` 返回一个统一 `Middleware`，可复用于 gRPC 与 HTTP。配置经
  `--mesh.middleware-route`（`selector=mw1,mw2`）注入，`options.ServerOptions.BuildMatcher` 解析。

## 5. 可观测（OTel）

`pkg/options/otel_options.go` 支持五态输出（`otel`/`file`/`console`/`classic`/`hybrid`），
三信号（trace/metric/log）统一接入 OTel，log 经 `pkg/otelslog` 桥接，`TraceIDHandler`
在 slog 记录中注入 trace/span id，实现 trace↔log 关联。
