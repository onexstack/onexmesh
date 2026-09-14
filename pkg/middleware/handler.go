// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onexstack/onexmesh/pkg/codec"
	"github.com/onexstack/onexstack/pkg/errorsx"
)

// HTTPHandler adapts a unified Handler into a gin.HandlerFunc, so a single
// business function can serve both gRPC and HTTP. It constructs the request
// value with newReq, decodes the body with codec.Bind, invokes the handler and
// renders the response with codec.Render. Errors are written as an ErrorX JSON
// envelope via codec.RenderError.
//
//	newReq returns a fresh request value (e.g. &pb.HelloRequest{}), or nil when
//	the endpoint has no request body and binding should be skipped.
func HTTPHandler(h Handler, newReq func() any) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := newReq()
		if req != nil {
			if err := codec.Bind(c, req); err != nil {
				codec.RenderError(c, errorsx.ErrBind.Wrap(err))
				return
			}
		}

		resp, err := h(c.Request.Context(), req)
		if err != nil {
			codec.RenderError(c, err)
			return
		}
		if resp == nil {
			c.Status(http.StatusOK)
			return
		}
		codec.Render(c, http.StatusOK, resp)
	}
}
