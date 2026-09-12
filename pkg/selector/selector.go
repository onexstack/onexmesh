// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package selector provides pluggable load-balancing over a set of nodes, with
// optional result feedback via DoneFunc so strategies can adapt to per-node
// health (strategy pattern + feedback loop).
package selector

import (
	"context"
	"fmt"
)

// Node abstracts a selectable instance.
type Node interface {
	// Address is the host:port to connect to.
	Address() string
	// Scheme is the transport scheme, e.g. "grpc" or "http".
	Scheme() string
	// ServiceName is the logical service name.
	ServiceName() string
	// Weight is the node's static weight (>=0).
	Weight() int
	// Metadata carries arbitrary tags.
	Metadata() map[string]string
}

// node is the default Node implementation.
type node struct {
	address     string
	scheme      string
	serviceName string
	weight      int
	metadata    map[string]string
}

// NewNode builds a Node from its parts.
func NewNode(address, scheme, serviceName string, weight int, metadata map[string]string) Node {
	return &node{
		address:     address,
		scheme:      scheme,
		serviceName: serviceName,
		weight:      weight,
		metadata:    metadata,
	}
}

func (n *node) Address() string             { return n.address }
func (n *node) Scheme() string              { return n.scheme }
func (n *node) ServiceName() string         { return n.serviceName }
func (n *node) Weight() int                 { return n.weight }
func (n *node) Metadata() map[string]string { return n.metadata }

// DoneInfo reports the outcome of a completed call on a selected node.
type DoneInfo struct {
	Err           error
	BytesSent     int64
	BytesReceived int64
}

// DoneFunc is called after a call completes so the selector can feed the
// result back into its decision making.
type DoneFunc func(ctx context.Context, di DoneInfo)

// Selector selects a node from the candidate set.
type Selector interface {
	// Select returns the chosen node and a DoneFunc to report the call result.
	// It returns an error when no node is available.
	Select(ctx context.Context, nodes []Node) (Node, DoneFunc, error)
}

// Factory constructs a Selector instance.
type Factory func() Selector

var factories = map[string]Factory{}

// RegisterSelector registers a selector Factory under name. Implementations
// call this from init().
func RegisterSelector(name string, f Factory) {
	factories[name] = f
}

// GetSelector returns a new Selector by name.
func GetSelector(name string) (Selector, error) {
	f, ok := factories[name]
	if !ok {
		return nil, fmt.Errorf("selector %q not registered", name)
	}
	return f(), nil
}

// Registered reports whether a selector strategy is registered under name.
func Registered(name string) bool {
	_, ok := factories[name]
	return ok
}

// NoopDone is a DoneFunc that discards feedback.
func NoopDone(ctx context.Context, di DoneInfo) {}
