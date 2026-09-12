// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// helloworld-client discovers the Greeter service by name and calls it over
// both gRPC and HTTP (Protobuf body).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	proto "github.com/onexstack/onexmesh/examples/helloworld/proto"
	"github.com/onexstack/onexmesh/pkg/client"
	"github.com/onexstack/onexmesh/pkg/codec"
	"github.com/onexstack/onexmesh/pkg/registry/consul"
	_ "github.com/onexstack/onexmesh/pkg/registry/consul"
	"github.com/onexstack/onexmesh/pkg/registry/etcd"
	_ "github.com/onexstack/onexmesh/pkg/registry/etcd"
	"github.com/onexstack/onexmesh/pkg/registry/eureka"
	_ "github.com/onexstack/onexmesh/pkg/registry/eureka"
	"github.com/onexstack/onexmesh/pkg/registry/nacos"
	_ "github.com/onexstack/onexmesh/pkg/registry/nacos"
	"github.com/onexstack/onexmesh/pkg/registry/polaris"
	_ "github.com/onexstack/onexmesh/pkg/registry/polaris"
)

func main() {
	var (
		serviceName  = flag.String("service", "edu.course.student-api", "Service name")
		registryType = flag.String("registry", "etcd", "Registry type: etcd, polaris, consul, nacos, eureka")
		registryAddr = flag.String("registry-addr", "127.0.0.1:2379", "Registry address")
		namespace    = flag.String("namespace", "default", "Namespace")
		name         = flag.String("name", "world", "Name to greet")
		useHTTP      = flag.Bool("http", false, "Call over HTTP (protobuf body) instead of gRPC")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if *useHTTP {
		callHTTP(ctx, *serviceName, *registryType, *registryAddr, *namespace, *name)
		return
	}

	conn, err := client.Dial(ctx, *serviceName,
		client.WithRegistry(*registryType, registryOpts(*registryType, *registryAddr, *namespace)),
		client.WithTimeout(5*time.Second),
	)
	if err != nil {
		slog.Error("dial", "err", err)
		return
	}
	defer conn.Close()

	grpcClient := proto.NewGreeterClient(conn)
	resp, err := grpcClient.SayHello(ctx, &proto.HelloRequest{Name: *name})
	if err != nil {
		slog.Error("say hello", "err", err)
		return
	}
	fmt.Println(resp.GetMessage())
}

func callHTTP(ctx context.Context, serviceName, regType, addr, namespace, name string) {
	httpClient, err := client.NewHTTPClient(serviceName,
		client.WithHTTPRegistry(regType, registryOpts(regType, addr, namespace)),
		client.WithHTTPCodec(codec.Proto{}),
		client.WithHTTPTimeout(5*time.Second),
	)
	if err != nil {
		slog.Error("new http client", "err", err)
		return
	}

	var resp proto.HelloReply
	if err := httpClient.Do(ctx, http.MethodPost, "/hello", &proto.HelloRequest{Name: name}, &resp); err != nil {
		slog.Error("http call", "err", err)
		return
	}
	fmt.Println(resp.GetMessage())
}

func registryOpts(regType, addr, namespace string) any {
	switch regType {
	case "etcd":
		return etcd.Options{Endpoints: []string{addr}, Namespace: namespace}
	case "polaris":
		return polaris.Options{Addr: addr, Namespace: namespace}
	case "consul":
		return consul.Options{Addr: addr}
	case "nacos":
		return nacos.Options{Addr: addr, NamespaceID: namespace}
	case "eureka":
		return eureka.Options{ServerURL: addr}
	default:
		return nil
	}
}
