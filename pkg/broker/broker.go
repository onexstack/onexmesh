// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package broker defines an asynchronous publish/subscribe abstraction with
// acknowledgement semantics, modeled after go-micro's Broker. It is the
// cross-process counterpart to pkg/event (which is process-local): a Broker
// moves messages between services, with at-least-once delivery when the
// implementation supports Ack.
package broker

import (
	"context"

	"github.com/onexstack/onexmesh/pkg/metadata"
)

// Message is a single broker message.
type Message struct {
	Header metadata.Metadata `json:"header,omitempty"`
	Body   []byte            `json:"body,omitempty"`
}

// Event is a delivered message received by a subscriber.
type Event interface {
	// Topic returns the topic this event was published to.
	Topic() string
	// Message returns the underlying message.
	Message() *Message
	// Ack acknowledges the message. For at-least-once brokers this commits the
	// message; for at-most-once brokers it is a no-op.
	Ack() error
	// Error returns a delivery error, if any.
	Error() error
}

// Handler processes a delivered event.
type Handler func(Event) error

// Subscriber is an active subscription.
type Subscriber interface {
	// Topic returns the subscribed topic.
	Topic() string
	// Unsubscribe cancels the subscription and releases its resources.
	Unsubscribe() error
}

// Broker publishes messages to topics and delivers them to subscribers.
type Broker interface {
	// Publish sends a message to topic.
	Publish(ctx context.Context, topic string, m *Message, opts ...PublishOption) error
	// Subscribe registers a handler for topic.
	Subscribe(ctx context.Context, topic string, h Handler, opts ...SubscribeOption) (Subscriber, error)
	// Connect establishes the underlying connection.
	Connect() error
	// Disconnect closes the underlying connection.
	Disconnect() error
	// String names the broker implementation.
	String() string
}

// PublishOption configures a Publish call.
type PublishOption func(*PublishOptions)

// PublishOptions holds Publish tunables.
type PublishOptions struct{}

// SubscribeOption configures a Subscribe call.
type SubscribeOption func(*SubscribeOptions)

// SubscribeOptions holds Subscribe tunables.
type SubscribeOptions struct {
	// Queue is a consumer-group name. Subscribers sharing a queue (and topic)
	// load-balance messages; otherwise each subscriber gets every message.
	Queue string
	// AutoAck acknowledges a message automatically when the handler returns nil.
	// When false, the handler must call Event.Ack explicitly.
	AutoAck bool
	// Concurrency is the number of goroutines consuming the subscription.
	Concurrency int
}

// WithQueue sets the consumer group name.
func WithQueue(queue string) SubscribeOption {
	return func(o *SubscribeOptions) { o.Queue = queue }
}

// WithAutoAck sets whether messages are auto-acknowledged on handler success.
func WithAutoAck(v bool) SubscribeOption {
	return func(o *SubscribeOptions) { o.AutoAck = v }
}

// WithConcurrency sets the number of concurrent consumers.
func WithConcurrency(n int) SubscribeOption {
	return func(o *SubscribeOptions) { o.Concurrency = n }
}
