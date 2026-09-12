// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package codec provides content-type-aware message marshalers for the HTTP
// transport, supporting JSON and Protobuf bodies.
package codec

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// Content types understood by the codec package.
const (
	ContentTypeJSON  = "application/json"
	ContentTypeProto = "application/x-protobuf"
)

// Marshaler marshals and unmarshals a message body and reports its content type.
type Marshaler interface {
	ContentType() string
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// JSON is the application/json codec.
type JSON struct{}

// ContentType returns application/json.
func (JSON) ContentType() string { return ContentTypeJSON }

// Marshal encodes v as JSON.
func (JSON) Marshal(v any) ([]byte, error) { return json.Marshal(v) }

// Unmarshal decodes JSON into v.
func (JSON) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// Proto is the application/x-protobuf codec.
type Proto struct{}

// ContentType returns application/x-protobuf.
func (Proto) ContentType() string { return ContentTypeProto }

// Marshal encodes a proto.Message into its wire format.
func (Proto) Marshal(v any) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("proto codec: value of type %T does not implement proto.Message", v)
	}
	return proto.Marshal(msg)
}

// Unmarshal decodes wire bytes into a proto.Message.
func (Proto) Unmarshal(data []byte, v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("proto codec: value of type %T does not implement proto.Message", v)
	}
	return proto.Unmarshal(data, msg)
}

// FromContentType returns the codec matching a request Content-Type header,
// defaulting to JSON for unknown or empty types.
func FromContentType(contentType string) Marshaler {
	switch contentType {
	case ContentTypeProto, "application/protobuf", "application/vnd.google.protobuf":
		return Proto{}
	default:
		return JSON{}
	}
}
