// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig configures CORS handling.
type CORSConfig struct {
	// AllowOrigins lists permitted origins. Empty means allow all ("*").
	AllowOrigins []string
	// AllowMethods lists permitted methods. Empty means a sensible default.
	AllowMethods []string
	// AllowHeaders lists permitted request headers. Empty means reflect the
	// request's Access-Control-Request-Headers.
	AllowHeaders []string
	// ExposeHeaders lists headers exposed to the browser.
	ExposeHeaders []string
	// AllowCredentials permits credentials (cookies, auth headers).
	AllowCredentials bool
	// MaxAge is the preflight cache duration in seconds.
	MaxAge int
}

// CORS returns a gin middleware implementing Cross-Origin Resource Sharing. It
// handles preflight OPTIONS requests and appends the relevant headers to actual
// responses.
func CORS(cfg CORSConfig) gin.HandlerFunc {
	methods := cfg.AllowMethods
	if len(methods) == 0 {
		methods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions}
	}

	origin := "*"
	if len(cfg.AllowOrigins) > 0 {
		origin = strings.Join(cfg.AllowOrigins, ", ")
	}
	expose := ""
	if len(cfg.ExposeHeaders) > 0 {
		expose = strings.Join(cfg.ExposeHeaders, ", ")
	}

	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		if cfg.AllowCredentials {
			h.Set("Access-Control-Allow-Credentials", "true")
		}
		if expose != "" {
			h.Set("Access-Control-Expose-Headers", expose)
		}

		if c.Request.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))

			allowHeaders := cfg.AllowHeaders
			if len(allowHeaders) == 0 {
				if reqHeaders := c.Request.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
					allowHeaders = []string{reqHeaders}
				}
			}
			if len(allowHeaders) > 0 {
				h.Set("Access-Control-Allow-Headers", strings.Join(allowHeaders, ", "))
			}
			if cfg.MaxAge > 0 {
				h.Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
			}

			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
