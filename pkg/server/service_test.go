// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server_test

import (
	"context"
	"net/http"
	"testing"

	"google.golang.org/grpc"

	"github.com/onexstack/onexmesh/pkg/server"
)

type testReq struct{ Name string }
type testResp struct{ Message string }

// fakeRegistrar captures the ServiceDesc passed to RegisterService.
type fakeRegistrar struct {
	desc *grpc.ServiceDesc
	impl any
}

func (f *fakeRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	f.desc = desc
	f.impl = impl
}

// TestNewMethodTypeErasure verifies the generic NewMethod erases Req/Resp behind
// the unified Handler: the correct request type invokes the function, a wrong
// type returns a clear error.
func TestNewMethodTypeErasure(t *testing.T) {
	m := server.NewMethod("SayHello", "/hello",
		func() any { return &testReq{} },
		func(_ context.Context, r *testReq) (*testResp, error) {
			return &testResp{Message: "Hello " + r.Name}, nil
		},
	)

	if m.Name != "SayHello" || m.Path != "/hello" || m.Method != http.MethodPost {
		t.Fatalf("method = %+v, want name=SayHello path=/hello method=POST", m)
	}

	resp, err := m.Handler(context.Background(), &testReq{Name: "world"})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got := resp.(*testResp).Message; got != "Hello world" {
		t.Fatalf("message = %q, want %q", got, "Hello world")
	}

	if _, err := m.Handler(context.Background(), "wrong-type"); err == nil {
		t.Fatal("handler with wrong request type = nil error, want error")
	}
}

// TestRegisterGRPC verifies the runtime-constructed ServiceDesc: correct service
// and method names, nil impl, and a method handler that decodes and invokes the
// unified Handler.
func TestRegisterGRPC(t *testing.T) {
	m := server.NewMethod("SayHello", "/hello",
		func() any { return &testReq{} },
		func(_ context.Context, r *testReq) (*testResp, error) {
			return &testResp{Message: "Hello " + r.Name}, nil
		},
	)
	svc := server.NewService("helloworld.Greeter", m)

	reg := &fakeRegistrar{}
	svc.RegisterGRPC(reg)

	if reg.desc == nil {
		t.Fatal("RegisterService not called")
	}
	if reg.desc.ServiceName != "helloworld.Greeter" {
		t.Fatalf("ServiceName = %q, want %q", reg.desc.ServiceName, "helloworld.Greeter")
	}
	if reg.impl != nil {
		t.Fatalf("impl = %v, want nil (to skip type-check)", reg.impl)
	}
	if len(reg.desc.Methods) != 1 || reg.desc.Methods[0].MethodName != "SayHello" {
		t.Fatalf("methods = %+v, want single SayHello", reg.desc.Methods)
	}

	handler := reg.desc.Methods[0].Handler
	dec := func(v any) error {
		v.(*testReq).Name = "world"
		return nil
	}
	resp, err := handler(nil, context.Background(), dec, nil)
	if err != nil {
		t.Fatalf("method handler: %v", err)
	}
	if got := resp.(*testResp).Message; got != "Hello world" {
		t.Fatalf("message = %q, want %q", got, "Hello world")
	}
}

// TestRegisterGRPCSkipsEmpty verifies a service without Methods does not register.
func TestRegisterGRPCSkipsEmpty(t *testing.T) {
	svc := server.NewService("empty.Service")
	reg := &fakeRegistrar{}
	svc.RegisterGRPC(reg)
	if reg.desc != nil {
		t.Fatal("empty service should not register a ServiceDesc")
	}
}
