// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onexstack/onexstack/pkg/errorsx"
)

// RenderError normalizes an arbitrary error into an *errorsx.ErrorX and writes
// it as a JSON envelope carrying its code/reason/message/metadata fields, with
// the matching HTTP status code. Error responses are always JSON: the gRPC
// transport carries the same error through GRPCStatus(), so a service returns
// one error and both protocols render it consistently.
func RenderError(c *gin.Context, err error) {
	x := errorsx.FromError(err)
	if x == nil {
		c.Status(http.StatusOK)
		return
	}
	// ErrorX declares json tags on Code/Reason/Message/Metadata, so the envelope
	// matches the errorsx contract without a hand-written DTO.
	c.JSON(x.Code, x)
}
