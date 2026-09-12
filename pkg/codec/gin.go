// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Bind reads the request body and unmarshals it into v using the codec selected
// by the request Content-Type. Unknown or empty content types fall back to JSON.
//
//	v := &pb.HelloRequest{}
//	if err := codec.Bind(c, v); err != nil { ... }
func Bind(c *gin.Context, v any) error {
	mc := FromContentType(c.ContentType())
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	return mc.Unmarshal(data, v)
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
