// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// compressionProbe is the body the tests serve. It is over the threshold and
// repetitive, the way the JSON this middleware exists for is: assert-compressible
// on purpose, so a test that fails is about the middleware and not about gzip
// deciding a particular string was incompressible.
var compressionProbe = strings.Repeat(`{"productID":"prd-course-linux-basic","name":"Linux 入门课程"},`, 40)

// serveCompress runs one request against a gin engine carrying Compress and a
// handler that writes what the test asked for.
func serveCompress(t *testing.T, handler gin.HandlerFunc, acceptEncoding, accept string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(Compress())
	engine.GET("/probe", handler)

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// jsonHandler writes body as application/json, declaring its length the way
// rest.Marshal does — which is what makes the size decision possible before the
// first byte is written.
func jsonHandler(body string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Length", strconv.Itoa(len(body)))
		c.String(http.StatusOK, body)
	}
}

// gunzip decodes a compressed body, failing the test if it is not gzip at all.
func gunzip(t *testing.T, body []byte) string {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("body is not gzip: %v", err)
	}
	defer func() { _ = reader.Close() }()

	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading the gzip body: %v", err)
	}
	return string(out)
}

// TestCompressEncodesWhenTheClientAsks is the feature: a large JSON response is
// compressed, and — the part that matters — still decodes to exactly what the
// handler wrote.
func TestCompressEncodesWhenTheClientAsks(t *testing.T) {
	rec := serveCompress(t, jsonHandler(compressionProbe), gzipEncoding, "")

	if got := rec.Header().Get("Content-Encoding"); got != gzipEncoding {
		t.Fatalf("Content-Encoding = %q, want %q", got, gzipEncoding)
	}
	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Fatalf("Content-Length = %q, want it dropped: it describes the uncompressed body and would truncate the compressed one", got)
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("Vary = %q, want it to name Accept-Encoding", rec.Header().Get("Vary"))
	}
	if got := gunzip(t, rec.Body.Bytes()); got != compressionProbe {
		t.Fatalf("decompressed body does not match: got %d bytes, want %d", len(got), len(compressionProbe))
	}
	if rec.Body.Len() >= len(compressionProbe) {
		t.Fatalf("compressed body is %d bytes, not smaller than the %d it replaced", rec.Body.Len(), len(compressionProbe))
	}
}

// TestCompressLeavesUnwillingClientsAlone covers the requests that must not be
// compressed. A wrong yes here is worse than a wrong no: the client receives a
// body it cannot decode.
func TestCompressLeavesUnwillingClientsAlone(t *testing.T) {
	tests := []struct {
		name           string
		acceptEncoding string
	}{
		{"no Accept-Encoding", ""},
		{"gzip refused with q=0", "gzip;q=0"},
		{"gzip refused with q=0.000", "gzip;q=0.000"},
		{"only a coding we do not speak", "br, zstd"},
		{"wildcard refused", "*;q=0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveCompress(t, jsonHandler(compressionProbe), tc.acceptEncoding, "")

			if got := rec.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("Content-Encoding = %q, want none for Accept-Encoding %q", got, tc.acceptEncoding)
			}
			if rec.Body.String() != compressionProbe {
				t.Fatalf("body was altered for Accept-Encoding %q", tc.acceptEncoding)
			}
			// Declared either way — a cache must not hand this response to a
			// client that would have got the compressed one.
			if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
				t.Fatalf("Vary = %q, want it to name Accept-Encoding", rec.Header().Get("Vary"))
			}
		})
	}
}

// TestCompressAcceptsTheWildcard is the other side of the q=0 cases: `*` is the
// client saying it takes anything, and gzip is something.
func TestCompressAcceptsTheWildcard(t *testing.T) {
	rec := serveCompress(t, jsonHandler(compressionProbe), "*", "")

	if got := rec.Header().Get("Content-Encoding"); got != gzipEncoding {
		t.Fatalf("Content-Encoding = %q, want %q", got, gzipEncoding)
	}
}

// TestCompressSkipsSmallBodies pins the threshold. Compressing a few hundred
// bytes produces a larger body and costs a writer, so the middleware must leave
// them alone even though the client asked.
func TestCompressSkipsSmallBodies(t *testing.T) {
	small := `{"code":0,"message":"ok"}` // well under DefaultMinCompressBytes
	rec := serveCompress(t, jsonHandler(small), gzipEncoding, "")

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want none for a %d-byte body", got, len(small))
	}
	if rec.Body.String() != small {
		t.Fatalf("body = %q, want it untouched", rec.Body.String())
	}
}

// TestCompressSkipsBodylessStatuses covers the statuses where a Content-Encoding
// would describe a body that is not there — the pair a 304 on a compressed
// resource otherwise looks like.
func TestCompressSkipsBodylessStatuses(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotModified} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			rec := serveCompress(t, func(c *gin.Context) {
				c.Status(status)
			}, gzipEncoding, "")

			if got := rec.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("Content-Encoding = %q, want none on %d", got, status)
			}
			if rec.Code != status {
				t.Fatalf("status = %d, want %d", rec.Code, status)
			}
		})
	}
}

// TestCompressLeavesAnEventStreamAlone is the regression this middleware is
// riskiest for, and it asserts the stronger of the two guarantees.
//
// Compressing a stream would defeat its only purpose — gzip cannot emit a block
// until it has enough input, so a poke would sit in the compressor instead of
// reaching the browser. The weaker guarantee (decide not to compress at
// WriteHeader) still wraps the writer, and a wrapped writer is a problem even
// when it passes every byte through: the streaming handler clears the
// connection's write deadline through http.NewResponseController(c.Writer), and
// the middleware is only safe for streams if that wrapper was never installed.
// So this asserts the writer is gin's own, not the middleware's.
func TestCompressLeavesAnEventStreamAlone(t *testing.T) {
	t.Run("because the request asks for a stream", func(t *testing.T) {
		var wrapped bool
		rec := serveCompress(t, func(c *gin.Context) {
			_, wrapped = c.Writer.(*compressWriter)
			c.Header("Content-Type", eventStreamMediaType)
			c.String(http.StatusOK, ": connected\n\n")
			c.Writer.Flush()
		}, gzipEncoding, eventStreamMediaType)

		if wrapped {
			t.Fatal("the streaming request's writer was wrapped; the handler can no longer clear the connection's write deadline")
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want none on a stream", got)
		}
		if rec.Body.String() != ": connected\n\n" {
			t.Fatalf("body = %q, want the frame unaltered", rec.Body.String())
		}
	})

	t.Run("because the response is a stream", func(t *testing.T) {
		// A client that omitted Accept — the second line of defence. The body
		// is over the threshold, so only the Content-Type stands between it and
		// the compressor.
		rec := serveCompress(t, func(c *gin.Context) {
			c.Header("Content-Type", eventStreamMediaType)
			c.String(http.StatusOK, compressionProbe)
		}, gzipEncoding, "")

		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want none on a stream", got)
		}
		if rec.Body.String() != compressionProbe {
			t.Fatal("the stream body was altered")
		}
	})
}

// lateLengthHandler is the header shape the services that consume this
// middleware actually produce.
//
// Their responses go through rest.Render → gin's c.Data, and gin v1.12's
// render.Data sets Content-Length *after* WriteHeader returns:
//
//	func (r Data) Render(w http.ResponseWriter) (err error) {
//		r.WriteContentType(w)
//		if len(r.Data) > 0 {
//			w.Header().Set("Content-Length", strconv.Itoa(len(r.Data)))
//		}
//		_, err = w.Write(r.Data)
//		return
//	}
//
// onexmesh's own test binary resolves gin v1.11, whose render.Data has no such
// line, so this handler states the shape explicitly rather than depending on
// which gin the test is linked against. See the comment on decide for why the
// difference matters.
func lateLengthHandler(body string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Writer.WriteHeader(http.StatusOK)
		c.Header("Content-Length", strconv.Itoa(len(body)))
		_, _ = c.Writer.Write([]byte(body))
	}
}

// TestCompressWithALateContentLength is the regression for deciding too early.
//
// Two things break at once if the middleware decides at WriteHeader rather than
// at the write. Content-Length is not set yet, so deleting it is a no-op — and
// the response goes out claiming the uncompressed length while carrying the
// compressed body, which a real client truncates or waits out. And the size
// threshold cannot see the length, so every small response gets compressed into
// a larger one.
func TestCompressWithALateContentLength(t *testing.T) {
	t.Run("large body is compressed and its length is withdrawn", func(t *testing.T) {
		rec := serveCompress(t, lateLengthHandler(compressionProbe), gzipEncoding, "")

		if got := rec.Header().Get("Content-Encoding"); got != gzipEncoding {
			t.Fatalf("Content-Encoding = %q, want %q", got, gzipEncoding)
		}
		// The assertion this test exists for.
		if got := rec.Header().Get("Content-Length"); got != "" {
			t.Fatalf("Content-Length = %q, want it withdrawn: the response would claim %s bytes and deliver %d", got, got, rec.Body.Len())
		}
		if got := gunzip(t, rec.Body.Bytes()); got != compressionProbe {
			t.Fatalf("decompressed body does not match: got %d bytes, want %d", len(got), len(compressionProbe))
		}
	})

	t.Run("small body is left alone", func(t *testing.T) {
		small := `{"code":0,"message":"ok"}`
		rec := serveCompress(t, lateLengthHandler(small), gzipEncoding, "")

		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want none: the threshold has to still see a Content-Length set after WriteHeader", got)
		}
		if rec.Body.String() != small {
			t.Fatalf("body = %q, want it untouched", rec.Body.String())
		}
		if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(small)) {
			t.Fatalf("Content-Length = %q, want it intact on an uncompressed response", got)
		}
	})
}

// TestCompressWithoutALengthCompresses covers the other half of that decision: a
// handler that never declares a length — gin v1.11's own renderer, or any
// streaming handler — cannot be judged on size, so it is judged on Content-Type
// alone and compressed.
func TestCompressWithoutALengthCompresses(t *testing.T) {
	small := `{"code":0,"message":"ok"}`
	rec := serveCompress(t, func(c *gin.Context) {
		c.String(http.StatusOK, small)
	}, gzipEncoding, "")

	if got := rec.Header().Get("Content-Encoding"); got != gzipEncoding {
		t.Fatalf("Content-Encoding = %q, want %q: with no Content-Length there is nothing to compare against the threshold", got, gzipEncoding)
	}
	if got := gunzip(t, rec.Body.Bytes()); got != small {
		t.Fatalf("decompressed body = %q, want %q", got, small)
	}
}

// TestCompressSurvivesAFlushingHandler checks the Flush override: a handler that
// flushes mid-response must still produce a body the client can decode, because
// the bytes it flushed have to have gone through the compressor.
func TestCompressSurvivesAFlushingHandler(t *testing.T) {
	rec := serveCompress(t, func(c *gin.Context) {
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Length", strconv.Itoa(len(compressionProbe)))
		_, _ = c.Writer.WriteString(compressionProbe[:len(compressionProbe)/2])
		c.Writer.Flush()
		_, _ = c.Writer.WriteString(compressionProbe[len(compressionProbe)/2:])
	}, gzipEncoding, "")

	if got := rec.Header().Get("Content-Encoding"); got != gzipEncoding {
		t.Fatalf("Content-Encoding = %q, want %q", got, gzipEncoding)
	}
	if got := gunzip(t, rec.Body.Bytes()); got != compressionProbe {
		t.Fatalf("decompressed body does not match after a mid-response flush: got %d bytes, want %d", len(got), len(compressionProbe))
	}
}

// TestAcceptsGzip is the negotiation table on its own, away from the writer.
func TestAcceptsGzip(t *testing.T) {
	tests := []struct {
		header string
		want   bool
	}{
		{"", false},
		{"gzip", true},
		{"GZIP", true},
		{" gzip ", true},
		{"gzip, deflate, br", true},
		{"deflate, gzip", true},
		{"gzip;q=0.5", true},
		{"gzip;q=0", false},
		{"gzip;q=0.0", false},
		{"br;q=1.0, gzip;q=0.1", true},
		{"br, zstd", false},
		{"*", true},
		{"*;q=0", false},
		{"br, *;q=0.5", true},
		// An explicit gzip beats the wildcard, so a refusal is a refusal.
		{"gzip;q=0, *", false},
	}

	for _, tc := range tests {
		t.Run(tc.header, func(t *testing.T) {
			if got := acceptsGzip(tc.header); got != tc.want {
				t.Fatalf("acceptsGzip(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}
