// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package redis provides a Redis Streams-backed broker.Broker with at-least-once
// delivery and explicit Ack. Subscribers sharing a queue form a consumer group
// that load-balances messages; unacknowledged messages can be reclaimed later.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/onexstack/onexmesh/pkg/broker"
	"github.com/onexstack/onexmesh/pkg/metadata"
)

func init() {
	broker.RegisterBroker("redis", func(opts any) (broker.Broker, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("redis: options must be %T, got %T", Options{}, opts)
		}
		return New(o)
	})
}

// Options configures the Redis broker.
type Options struct {
	// Addr is the Redis address used when Client is nil.
	Addr string
	// Client is a preconfigured Redis client.
	Client redis.UniversalClient
}

// New returns a Redis Streams Broker.
func New(opts Options) (broker.Broker, error) {
	client := opts.Client
	if client == nil {
		client = redis.NewClient(&redis.Options{Addr: opts.Addr})
	}
	return &redisBroker{client: client}, nil
}

type redisBroker struct {
	client redis.UniversalClient
}

type redisSubscriber struct {
	topic  string
	group  string
	cancel context.CancelFunc
	once   sync.Once
}

type streamEvent struct {
	topic string
	msg   *broker.Message
	ack   func() error
}

func (e *streamEvent) Topic() string            { return e.topic }
func (e *streamEvent) Message() *broker.Message { return e.msg }
func (e *streamEvent) Ack() error               { return e.ack() }
func (e *streamEvent) Error() error             { return nil }

func (b *redisBroker) String() string { return "redis" }

func (b *redisBroker) Connect() error {
	return b.client.Ping(context.Background()).Err()
}

func (b *redisBroker) Disconnect() error {
	return b.client.Close()
}

func (b *redisBroker) Publish(ctx context.Context, topic string, m *broker.Message, _ ...broker.PublishOption) error {
	header, err := json.Marshal(m.Header)
	if err != nil {
		return err
	}
	return b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: topic,
		Values: map[string]any{
			"header": string(header),
			"body":   string(m.Body),
		},
	}).Err()
}

func (b *redisBroker) Subscribe(ctx context.Context, topic string, h broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	o := broker.SubscribeOptions{AutoAck: true, Concurrency: 1}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Concurrency < 1 {
		o.Concurrency = 1
	}
	group := o.Queue
	if group == "" {
		group = "default"
	}
	consumer := fmt.Sprintf("consumer-%d", time.Now().UnixNano())

	// Create the consumer group from the newest message. BUSYGROUP is expected
	// on re-subscribe and is ignored; any other error (e.g. connection) is fatal.
	err := b.client.XGroupCreateMkStream(ctx, topic, group, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return nil, fmt.Errorf("redis: create group %q: %w", group, err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	s := &redisSubscriber{topic: topic, group: group, cancel: cancel}

	for i := 0; i < o.Concurrency; i++ {
		go b.consume(subCtx, topic, group, consumer, h, o)
	}
	return s, nil
}

func (b *redisBroker) consume(ctx context.Context, topic, group, consumer string, h broker.Handler, o broker.SubscribeOptions) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{topic, ">"},
			Count:    1,
			Block:    5 * time.Second,
		}).Result()
		if err != nil {
			if err == redis.Nil || ctx.Err() != nil {
				continue
			}
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				ev := &streamEvent{
					topic: topic,
					msg:   decodeMessage(msg.Values),
					ack: func() error {
						return b.client.XAck(context.Background(), topic, group, msg.ID).Err()
					},
				}

				if gerr := h(ev); gerr == nil && o.AutoAck {
					_ = ev.Ack()
				}
				// On handler error (or manual Ack), leave the message pending so
				// an XAUTOCLAIM pass can redeliver it later.
			}
		}
	}
}

func (s *redisSubscriber) Topic() string { return s.topic }

func (s *redisSubscriber) Unsubscribe() error {
	s.once.Do(func() { s.cancel() })
	return nil
}

func decodeMessage(values map[string]any) *broker.Message {
	m := &broker.Message{Header: metadata.Metadata{}}
	if h, ok := values["header"].(string); ok && h != "" {
		_ = json.Unmarshal([]byte(h), &m.Header)
	}
	if b, ok := values["body"].(string); ok {
		m.Body = []byte(b)
	}
	return m
}
