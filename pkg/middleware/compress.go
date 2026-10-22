// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Compression defaults.
const (
	// DefaultMinCompressBytes is the smallest response this middleware will
	// compress.
	//
	// gzip has a fixed overhead (header, trailer, and the table it builds), so
	// compressing a 200-byte JSON error can produce a body larger than the
	// original — and it costs a writer allocation to do it. One kilobyte is
	// where the two curves cross for the JSON these services emit; below it the
	// middleware passes the response through untouched.
	DefaultMinCompressBytes = 1024

	// gzipEncoding is the only content-coding this middleware negotiates.
	gzipEncoding = "gzip"

	// eventStreamMediaType is never compressed, on either side of the decision:
	// see Compress.
	eventStreamMediaType = "text/event-stream"
)

// gzipWriterPool recycles gzip writers.
//
// A gzip.Writer holds a compressor with roughly 300KB of tables; allocating one
// per response would make a 2KB JSON response cost more in garbage than in
// bytes. Reset clears the writer's state but keeps the tables, which is the
// whole reason to pool them.
var gzipWriterPool = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

// CompressionConfig configures Compress.
type CompressionConfig struct {
	// MinBytes overrides DefaultMinCompressBytes. Values <= 0 mean the default.
	MinBytes int
}

// Compress returns a gin middleware that gzip-encodes JSON responses when the
// client asks for it.
//
// # Why this exists
//
// Every REST response this platform serves is uncompressed, and the largest of
// them are not small: the console's product catalog is 117KB of mostly-empty
// JSON fields, and it is fetched on every page load by the top navigation. On a
// link slow enough for that to matter, compression is the difference between the
// navigation appearing and the page looking hung. JSON like this compresses
// about 4.5x.
//
// # Why it wraps the writer lazily instead of checking the route
//
// Whether a response may be compressed is a property of the response, not of
// the route: `Content-Type`, `Content-Length` and any existing
// `Content-Encoding` are all set by the handler, after this middleware has
// already run. Matching on a path suffix (`/stream`) would be a second,
// silently-drifting copy of the routing table. So the middleware wraps the
// writer and decides at the first WriteHeader, when the handler has said what it
// is about to send.
//
// # What must never be compressed: SSE
//
// A `text/event-stream` response is a sequence of frames whose entire value is
// that each one reaches the client as it is written. gzip buffers — it cannot
// emit a block until it has enough input — so compressing a stream converts
// "the browser is told the moment something changes" into "the browser is told
// once enough has happened". Every stream route on this platform
// (`/v1/notifications/stream`, `/v1/comments/stream`, `/v1/orders/{id}/stream`,
// `/v1/membership/stream`) would break that way.
//
// That is guarded twice, and the first guard is the one that matters: a request
// whose `Accept` names `text/event-stream` is not wrapped at all. Wrapping it
// and unwrapping at WriteHeader would work for the bytes and still be wrong for
// the connection — the streaming handler reaches the socket through
// `http.NewResponseController(c.Writer)` to clear the server's write deadline
// (see ssebus.detachConnDeadlines), and a wrapper that does not forward that is
// a stream the server cuts off at its own deadline. Not wrapping means the
// streaming path sees exactly the writer it sees today. The Content-Type check
// at WriteHeader stays as a second line of defence for a client that omits
// Accept.
func Compress() gin.HandlerFunc { return CompressWith(CompressionConfig{}) }

// CompressWith is Compress with an explicit configuration.
func CompressWith(cfg CompressionConfig) gin.HandlerFunc {
	minBytes := cfg.MinBytes
	if minBytes <= 0 {
		minBytes = DefaultMinCompressBytes
	}

	return func(c *gin.Context) {
		// Declared on every path, compressed or not: a cache that stores the
		// uncompressed variant and serves it to a client that asked for gzip is
		// the failure this header exists to prevent.
		c.Writer.Header().Add("Vary", "Accept-Encoding")

		if !acceptsGzip(c.Request.Header.Get("Accept-Encoding")) ||
			prefersEventStream(c.Request.Header.Get("Accept")) {
			c.Next()
			return
		}

		w := &compressWriter{ResponseWriter: c.Writer, minBytes: minBytes}
		c.Writer = w
		c.Next()
		// After the chain, not deferred before it: the handler writes the body
		// during c.Next(), and the gzip trailer is only produced by Close.
		w.close()
	}
}

// compressWriter decides once, at the last moment before the header is
// committed, whether this response is compressed, and then behaves as a plain
// writer either way.
//
// It embeds gin.ResponseWriter so Status, Size, Written, Hijack and CloseNotify
// keep working through it without being restated here; only the methods that
// carry bytes or the decision are overridden.
type compressWriter struct {
	gin.ResponseWriter

	minBytes int

	// gz is non-nil only once the response has been judged compressible.
	gz *gzip.Writer
	// decided records that the judgement has been made, so it is made once and
	// not re-made (nor the pool consulted) on every subsequent write.
	decided bool
	// status is what WriteHeader was told, needed by the decision.
	status int
}

// WriteHeader records the status and gets out of the way.
//
// It deliberately does not decide here. gin's WriteHeader only records a status;
// the header itself is committed on the first write to the socket. And the two
// facts the decision needs are set *after* this call returns: gin v1.12's
// render.Data — the renderer behind every one of these services' successful
// responses, via rest.Render's c.Data — sets Content-Length immediately after
// c.Status, and the body length is half the judgement. Deciding here would miss
// it, which costs twice: the Content-Length the middleware meant to remove is
// not there yet to remove, and the size threshold can never fire.
func (w *compressWriter) WriteHeader(code int) {
	if w.decided {
		w.ResponseWriter.WriteHeader(code)
		return
	}

	w.status = code
	if !bodyCarrying(code) {
		// Decidable now, and worth deciding now: a 204 must not acquire a
		// Content-Encoding no matter what else happens.
		w.decided = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// WriteHeaderNow is gin's commit point, and is overridden so a caller that
// reaches it directly still gets a decided response.
func (w *compressWriter) WriteHeaderNow() {
	w.decide()
	w.ResponseWriter.WriteHeaderNow()
}

// decide makes the compress-or-not call exactly once, at the last moment before
// the header leaves.
func (w *compressWriter) decide() {
	if w.decided {
		return
	}
	w.decided = true

	if w.status == 0 {
		w.status = http.StatusOK
	}
	if !w.compressible() {
		return
	}

	header := w.ResponseWriter.Header()
	header.Set("Content-Encoding", gzipEncoding)
	// The body is about to change size, so a Content-Length describing the
	// uncompressed one is now a lie — and a lie a client would believe, reading
	// 117KB of promised body and truncating at the 26KB it receives. Removing
	// it lets net/http switch to chunked encoding, which is what a compressed
	// response is.
	header.Del("Content-Length")

	w.gz = gzipWriterPool.Get().(*gzip.Writer)
	w.gz.Reset(w.ResponseWriter)
}

// compressible reports whether a response with this status and these headers may
// be compressed. It is only meaningful once the handler has finished setting
// headers, which is why decide defers it.
func (w *compressWriter) compressible() bool {
	if !bodyCarrying(w.status) {
		return false
	}

	header := w.ResponseWriter.Header()
	// Already encoded by someone else (a handler that compressed, or a proxy
	// that set it upstream). Encoding it twice is not compression, it is
	// corruption.
	if header.Get("Content-Encoding") != "" {
		return false
	}
	// The SSE guard's second line; see Compress for why the first one is the
	// one that protects the stream.
	if strings.HasPrefix(strings.ToLower(header.Get("Content-Type")), eventStreamMediaType) {
		return false
	}
	// Only knowable when the handler declared a length. A response without one
	// is streamed and is judged on Content-Type alone.
	if length := header.Get("Content-Length"); length != "" {
		if n, err := strconv.Atoi(length); err == nil && n < w.minBytes {
			return false
		}
	}
	return true
}

// bodyCarrying reports whether a status may carry a body — the pair where a
// Content-Encoding would otherwise describe something that is not there.
func bodyCarrying(code int) bool {
	return code != http.StatusNoContent && code != http.StatusNotModified
}

// Write compresses the body, making the decision first.
func (w *compressWriter) Write(b []byte) (int, error) {
	w.decide()
	if w.gz == nil {
		return w.ResponseWriter.Write(b)
	}
	return w.gz.Write(b)
}

// WriteString must be overridden rather than inherited: gin's own
// ResponseWriter implements it by writing the string straight to the socket,
// which would bypass the compressor and emit a body the Content-Encoding header
// now describes wrongly.
func (w *compressWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Flush forces the decision, because a flush is a commit: whatever the
// compressor is holding has to go through it, and the header has to be right
// before it does.
func (w *compressWriter) Flush() {
	w.decide()
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	w.ResponseWriter.Flush()
}

// Unwrap exposes the wrapped writer to http.NewResponseController, so a
// deadline or hijack set through this wrapper reaches the same connection it
// would have reached without it.
func (w *compressWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// close finishes the gzip stream and returns the writer to the pool. Safe to
// call when nothing was compressed, and when nothing was written at all: an
// unstarted response is left alone rather than given a Content-Encoding over an
// empty body.
func (w *compressWriter) close() {
	if w.gz == nil {
		return
	}
	_ = w.gz.Close()
	gzipWriterPool.Put(w.gz)
	w.gz = nil
}

// acceptsGzip reports whether the request's Accept-Encoding permits gzip.
//
// The q-value matters and is not decoration: `Accept-Encoding: gzip;q=0` is a
// client saying it does not accept gzip, and treating it as acceptance makes the
// response unreadable to exactly the client that was careful enough to say so.
func acceptsGzip(header string) bool {
	if header == "" {
		// Absent means "no preference" in theory and "do not compress" from
		// every real client; the conservative reading is the correct one.
		return false
	}

	gzipQ, wildcardQ := -1.0, -1.0
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		name := strings.ToLower(strings.TrimSpace(fields[0]))

		q := 1.0
		for _, param := range fields[1:] {
			value, ok := strings.CutPrefix(strings.ToLower(strings.TrimSpace(param)), "q=")
			if !ok {
				continue
			}
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				q = parsed
			}
		}

		switch name {
		case gzipEncoding:
			gzipQ = q
		case "*":
			wildcardQ = q
		}
	}

	if gzipQ >= 0 {
		return gzipQ > 0
	}
	// `gzip` absent but `*` present: gzip is acceptable under the wildcard.
	return wildcardQ > 0
}

// prefersEventStream reports whether the client is asking for a Server-Sent
// Events stream. See Compress for why this is checked before wrapping.
func prefersEventStream(accept string) bool {
	return strings.Contains(strings.ToLower(accept), eventStreamMediaType)
}
