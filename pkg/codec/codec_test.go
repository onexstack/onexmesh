// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestJSONRoundTrip(t *testing.T) {
	c := JSON{}
	type obj struct {
		Name string `json:"name"`
	}

	b, err := c.Marshal(obj{Name: "onex"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out obj
	if err := c.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Name != "onex" {
		t.Fatalf("got %q, want %q", out.Name, "onex")
	}
}

func TestProtoRoundTrip(t *testing.T) {
	c := Proto{}

	b, err := c.Marshal(wrapperspb.String("hello"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out wrapperspb.StringValue
	if err := c.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.GetValue() != "hello" {
		t.Fatalf("got %q, want %q", out.GetValue(), "hello")
	}
}

func TestProtoRejectsNonMessage(t *testing.T) {
	c := Proto{}
	if _, err := c.Marshal("not-a-message"); err == nil {
		t.Fatal("expected error marshaling a non-proto.Message")
	}
	if err := c.Unmarshal([]byte("x"), "not-a-message"); err == nil {
		t.Fatal("expected error unmarshaling into a non-proto.Message")
	}
}

func TestFromContentType(t *testing.T) {
	if _, ok := FromContentType(ContentTypeProto).(Proto); !ok {
		t.Fatalf("expected Proto codec for %q", ContentTypeProto)
	}
	if _, ok := FromContentType(ContentTypeJSON).(JSON); !ok {
		t.Fatalf("expected JSON codec for %q", ContentTypeJSON)
	}
	if _, ok := FromContentType("application/octet-stream").(JSON); !ok {
		t.Fatal("expected JSON fallback for unknown content type")
	}
}
