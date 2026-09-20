// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package codec provides content-type-aware message marshalers for the HTTP
// transport, supporting JSON and Protobuf bodies.
package codec

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
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

// JSON is the application/json codec, backed by encoding/json.
//
// # This is not the codec a protobuf client wants
//
// For a value that implements proto.Message this codec is wrong in both
// directions, and wrong *late*: encoding/json and protobuf-JSON disagree about
// every well-known type. A google.protobuf.Timestamp is an RFC3339 string on the
// wire and a {Seconds, Nanos} struct in Go, so decoding one fails on the first
// message that actually carries a date —
//
//	json: cannot unmarshal string into Go struct field
//	  Membership.membership.createdAt of type timestamppb.Timestamp
//
// — while an unset timestamp is `null` and decodes cleanly. A client that works
// against empty data and breaks on real data is the worst shape a defect has.
// Duration ("1.5s"), FieldMask, Struct/Value and the wrapper types have the same
// problem, and marshalling goes the other way: protojson servers do not accept
// {"seconds":…} for a timestamp.
//
// A client that talks to a protobuf server should use ProtoJSON instead. This
// codec stays the default for the server's own request binding, where it has
// been in use across every route and where the contract's request messages carry
// no well-known types — changing that default would alter the accepted wire
// format of 284 endpoints to fix a client-side defect. See docs/fix.md.
type JSON struct{}

// ContentType returns application/json.
func (JSON) ContentType() string { return ContentTypeJSON }

// Marshal encodes v as JSON.
func (JSON) Marshal(v any) ([]byte, error) { return json.Marshal(v) }

// Unmarshal decodes JSON into v.
func (JSON) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// ProtoJSON is the application/json codec backed by protojson.
//
// It is the codec for talking to a service whose messages are protobuf: the wire
// format is protobuf-JSON, and this is the decoder that reads it. Use it for any
// protobuf request or response — see JSON for what goes wrong otherwise.
//
// # It reads this platform's responses, which protojson would not write
//
// protojson's parser accepts more than its writer emits, and one of the
// differences is load-bearing here: the contract types 64-bit integers as JSON
// numbers (see internal/pkg/rest.Marshal), where protojson writes them as quoted
// strings. protojson parses both forms, so a client using this codec reads
// `"price":399` and `"price":"399"` alike.
//
// Unknown fields are rejected, matching protojson's default. That is the right
// default for a typed client: a field the server sends and the client's IDL does
// not know is a version skew worth reporting, not something to drop.
type ProtoJSON struct{}

// ContentType returns application/json: this is a JSON encoding of a protobuf
// message, not the binary format.
func (ProtoJSON) ContentType() string { return ContentTypeJSON }

// Marshal encodes a proto.Message as protobuf-JSON.
func (ProtoJSON) Marshal(v any) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("protojson codec: value of type %T does not implement proto.Message", v)
	}
	return protojson.MarshalOptions{}.Marshal(msg)
}

// Unmarshal decodes protobuf-JSON into a proto.Message.
func (ProtoJSON) Unmarshal(data []byte, v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("protojson codec: value of type %T does not implement proto.Message", v)
	}
	return protojson.UnmarshalOptions{}.Unmarshal(data, msg)
}

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
