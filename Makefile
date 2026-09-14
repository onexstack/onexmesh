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

# proto generates the framework extension types (http.proto) and the example.
proto: proto-install
	$(PROTOC) \
		--proto_path="$(PROTOC_INCLUDE)" \
		--proto_path=. \
		--proto_path=pkg/proto \
		--go_out=. --go_opt=module=github.com/onexstack/onexmesh \
		pkg/proto/onexmesh/v1/http.proto
	$(PROTOC) \
		--proto_path="$(PROTOC_INCLUDE)" \
		--proto_path=. \
		--proto_path=pkg/proto \
		--go_out=. --go_opt=module=github.com/onexstack/onexmesh \
		--go-grpc_out=. --go-grpc_opt=module=github.com/onexstack/onexmesh \
		--onexmesh_out=. --onexmesh_opt=module=github.com/onexstack/onexmesh \
		examples/helloworld/proto/hello.proto
