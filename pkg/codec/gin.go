// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

// Bind reads the request body and unmarshals it into a protobuf message using
// the codec selected by the request Content-Type. Unknown or empty content types
// fall back to JSON.
//
//	req := &pb.HelloRequest{}
//	if err := codec.Bind(c, req); err != nil { ... }
func Bind(c *gin.Context, msg proto.Message) error {
	mc := FromContentType(c.ContentType())
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	return mc.Unmarshal(data, msg)
}

// BindHTTP decodes a protobuf request message from the full HTTP surface in one
// place: path parameters (from the "{field}" placeholders in template), query
// parameters, and the request body when body == "*". It is the single binding
// entry point behind server.Method.httpHandler, replacing the per-method
// hand-written BindPath/BindQuery/Bind sequences that generated code used to
// emit.
func BindHTTP(c *gin.Context, msg proto.Message, template, body string) error {
	if template != "" {
		if err := BindPath(c, msg, template); err != nil {
			return err
		}
	}
	if err := BindQuery(c, msg); err != nil {
		return err
	}
	if body == "*" {
		if err := Bind(c, msg); err != nil {
			return err
		}
	}
	return nil
}

// Render marshals v using the codec selected by the Accept header and writes it
// with the matching Content-Type. Unknown or empty Accept falls back to JSON.
// On a marshal error it aborts with HTTP 500.
//
//	codec.Render(c, http.StatusOK, &pb.HelloReply{Message: "hi"})
func Render(c *gin.Context, status int, v any) {
	mc := FromAccept(c.GetHeader("Accept"))
	data, err := mc.Marshal(v)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Data(status, mc.ContentType(), data)
}

// FromAccept returns the codec matching an Accept header value, defaulting to
// JSON when it does not request protobuf.
func FromAccept(accept string) Marshaler {
	if strings.Contains(accept, "protobuf") || strings.Contains(accept, "octet-stream") {
		return Proto{}
	}
	return JSON{}
}
