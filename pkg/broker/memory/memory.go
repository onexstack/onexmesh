// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package memory provides an in-process broker.Broker for tests and single-node
// deployments. Delivery is at-most-once and synchronous: Publish invokes
// subscribers on the caller's goroutine.
package memory

import (
	"context"
	"sync"

	"github.com/onexstack/onexmesh/pkg/broker"
)

func init() {
	broker.RegisterBroker("memory", func(any) (broker.Broker, error) { return New(), nil })
}

type memoryBroker struct {
	mu   sync.RWMutex
	subs map[string][]*subscription
}

type subscription struct {
	topic   string
	handler broker.Handler
	autoAck bool
	once    sync.Once
	done    chan struct{}
}

type event struct {
	topic string
	msg   *broker.Message
}

func (e *event) Topic() string            { return e.topic }
func (e *event) Message() *broker.Message { return e.msg }
func (e *event) Ack() error               { return nil } // at-most-once: no-op
func (e *event) Error() error             { return nil }

// New returns an in-process Broker.
func New() broker.Broker {
	return &memoryBroker{subs: make(map[string][]*subscription)}
}

func (b *memoryBroker) String() string { return "memory" }

func (b *memoryBroker) Connect() error    { return nil }
func (b *memoryBroker) Disconnect() error { return nil }

func (b *memoryBroker) Publish(_ context.Context, topic string, m *broker.Message, _ ...broker.PublishOption) error {
	b.mu.RLock()
	subs := append([]*subscription(nil), b.subs[topic]...)
	b.mu.RUnlock()

	for _, s := range subs {
		select {
		case <-s.done:
			continue
		default:
		}
		if err := s.handler(&event{topic: topic, msg: m}); err != nil {
			return err
		}
	}
	return nil
}

func (b *memoryBroker) Subscribe(_ context.Context, topic string, h broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	o := broker.SubscribeOptions{AutoAck: true}
	for _, opt := range opts {
		opt(&o)
	}

	s := &subscription{topic: topic, handler: h, autoAck: o.AutoAck, done: make(chan struct{})}
	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], s)
	b.mu.Unlock()
	return s, nil
}

func (s *subscription) Topic() string { return s.topic }

func (s *subscription) Unsubscribe() error {
	s.once.Do(func() { close(s.done) })
	return nil
}
