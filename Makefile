# Makefile for onexmesh

SHELL := /bin/bash
GO := go
GOFLAGS ?=

PROTOC ?= protoc
PROTOC_INCLUDE ?= $(shell dirname $$(dirname $$(command -v protoc)))/include
PROTOC_GO := $(shell go env GOPATH)/bin

.PHONY: all build test lint vet fmt clean proto proto-install

all: build

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run

clean:
	$(GO) clean

# proto-install builds and installs the protoc plugins.
proto-install:
	$(GO) install ./cmd/protoc-gen-errno
	$(GO) install ./cmd/protoc-gen-onexmesh
	$(GO) install ./cmd/protoc-gen-onexmesh-client

# proto generates the framework extension types (http/onexmesh/rest/meta.proto)
# and the examples.
proto: proto-install
	$(PROTOC) \
		--proto_path="$(PROTOC_INCLUDE)" \
		--proto_path=. \
		--proto_path=pkg/proto \
		--go_out=. --go_opt=module=github.com/onexstack/onexmesh \
		pkg/proto/onexmesh/v1/http.proto \
		pkg/proto/onexmesh/v1/onexmesh.proto \
		pkg/proto/onexmesh/rest/v1/rest.proto \
		pkg/proto/onexmesh/meta/v1/meta.proto
	$(PROTOC) \
		--proto_path="$(PROTOC_INCLUDE)" \
		--proto_path=. \
		--proto_path=pkg/proto \
		--go_out=. --go_opt=module=github.com/onexstack/onexmesh \
		--go-grpc_out=. --go-grpc_opt=module=github.com/onexstack/onexmesh \
		--onexmesh_out=. --onexmesh_opt=module=github.com/onexstack/onexmesh \
		examples/helloworld/proto/hello.proto
	# The client SDK example: resources + anchor are passed in one invocation so
	# the plugin can aggregate them into a single clientset, plus the gRPC
	# service that requests mesh service-discovery code generation.
	$(PROTOC) \
		--proto_path="$(PROTOC_INCLUDE)" \
		--proto_path=. \
		--proto_path=pkg/proto \
		--go_out=. --go_opt=module=github.com/onexstack/onexmesh \
		--go-grpc_out=. --go-grpc_opt=module=github.com/onexstack/onexmesh \
		--onexmesh-client_out=. --onexmesh-client_opt=module=github.com/onexstack/onexmesh \
		examples/apis/apps/v1/deployment.proto \
		examples/apis/apps/v1/service.proto \
		examples/pkg/generated/exampleclient/clientset.proto
