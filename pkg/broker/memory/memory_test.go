// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package memory

import (
	"context"
	"testing"

	"github.com/onexstack/onexmesh/pkg/broker"
	"github.com/onexstack/onexmesh/pkg/metadata"
)

func TestPublishSubscribe(t *testing.T) {
	b := New()

	got := make(chan *broker.Message, 1)
	sub, err := b.Subscribe(context.Background(), "topic", func(ev broker.Event) error {
		got <- ev.Message()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	if err := b.Publish(context.Background(), "topic", &broker.Message{Body: []byte("hello")}); err != nil {
		t.Fatal(err)
	}

	msg := <-got
	if string(msg.Body) != "hello" {
		t.Fatalf("body = %q, want %q", msg.Body, "hello")
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	b := New()

	var calls int
	sub, err := b.Subscribe(context.Background(), "topic", func(broker.Event) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_ = b.Publish(context.Background(), "topic", &broker.Message{})
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}

	if err := sub.Unsubscribe(); err != nil {
		t.Fatal(err)
	}
	_ = b.Publish(context.Background(), "topic", &broker.Message{})
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 after unsubscribe", calls)
	}
}

func TestEventHeader(t *testing.T) {
	b := New()
	got := make(chan broker.Event, 1)
	_, _ = b.Subscribe(context.Background(), "t", func(ev broker.Event) error {
		got <- ev
		return nil
	})

	_ = b.Publish(context.Background(), "t", &broker.Message{
		Header: metadata.Metadata{"k": "v"},
		Body:   []byte("b"),
	})

	ev := <-got
	if ev.Topic() != "t" {
		t.Fatalf("topic = %q, want t", ev.Topic())
	}
	if v, _ := ev.Message().Header.Get("k"); v != "v" {
		t.Fatalf("header[k] = %q, want v", v)
	}
	if err := ev.Ack(); err != nil {
		t.Fatalf("Ack() err = %v (memory Ack is a no-op)", err)
	}
}
