# Makefile for onexmesh

SHELL := /bin/bash
GO := go
GOFLAGS ?=

.PHONY: all build test lint vet fmt clean

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
