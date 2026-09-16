// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/trace"
)

var _ IOptions = (*SlogOptions)(nil)

// SlogOptions configures the slog logger.
type SlogOptions struct {
	Level      string `mapstructure:"level"`
	AddSource  bool   `mapstructure:"add-source"`
	Format     string `mapstructure:"format"`
	TimeFormat string `mapstructure:"time-format"`
	Output     string `mapstructure:"output"`

	mu     sync.Mutex
	closer io.Closer
}

// NewSlogOptions returns default slog options.
func NewSlogOptions() *SlogOptions {
	return &SlogOptions{
		Level:      "info",
		AddSource:  false,
		Format:     "text",
		TimeFormat: time.RFC3339Nano,
		Output:     "stdout",
	}
}

func (o *SlogOptions) Validate() []error {
	var errs []error
	switch strings.ToLower(o.Level) {
	case "debug", "info", "warn", "warning", "error":
	default:
		errs = append(errs, fmt.Errorf("invalid log level %q", o.Level))
	}
	if o.Format != "json" && o.Format != "text" {
		errs = append(errs, fmt.Errorf("invalid log format %q", o.Format))
	}
	return errs
}

func (o *SlogOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringVar(&o.Level, prefix+".level", o.Level, "Log level: debug, info, warn, error.")
	fs.StringVar(&o.Format, prefix+".format", o.Format, "Log format: json or text.")
	fs.BoolVar(&o.AddSource, prefix+".add-source", o.AddSource, "Add source file:line to logs.")
	fs.StringVar(&o.TimeFormat, prefix+".time-format", o.TimeFormat, "Time format for text logs.")
	fs.StringVar(&o.Output, prefix+".output", o.Output, "Log output: stdout, stderr or file path.")
}

// ToSlogLevel converts the level string to slog.Level.
func (o *SlogOptions) ToSlogLevel() slog.Level {
	switch strings.ToLower(o.Level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (o *SlogOptions) writer() (io.Writer, error) {
	switch o.Output {
	case "", "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		f, err := os.OpenFile(o.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		// Track the file so Shutdown can close it; close any previous file if the
		// logger is rebuilt (re-Apply).
		o.mu.Lock()
		if o.closer != nil {
			_ = o.closer.Close()
		}
		o.closer = f
		o.mu.Unlock()
		return f, nil
	}
}

// Shutdown closes any output file opened by the writer. It is idempotent.
func (o *SlogOptions) Shutdown() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closer == nil {
		return nil
	}
	err := o.closer.Close()
	o.closer = nil
	return err
}

// BuildHandler builds a slog.Handler from the options.
func (o *SlogOptions) BuildHandler() (slog.Handler, error) {
	w, err := o.writer()
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level:     o.ToSlogLevel(),
		AddSource: o.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				return slog.String(slog.TimeKey, a.Value.Time().Format(o.TimeFormat))
			case slog.MessageKey:
				return slog.String("message", a.Value.String())
			}
			return a
		},
	}

	var handler slog.Handler
	if o.Format == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	return &TraceIDHandler{Handler: handler}, nil
}

// BuildLogger builds a slog.Logger without touching the global logger.
func (o *SlogOptions) BuildLogger() (*slog.Logger, error) {
	h, err := o.BuildHandler()
	if err != nil {
		return nil, err
	}
	return slog.New(h), nil
}

// Apply sets the global default slog logger.
func (o *SlogOptions) Apply() error {
	logger, err := o.BuildLogger()
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	return nil
}

// TraceIDHandler decorates a slog.Handler, attaching the current trace/span
// ids from the request context to every record.
type TraceIDHandler struct {
	slog.Handler
}

func (h *TraceIDHandler) Handle(ctx context.Context, r slog.Record) error {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *TraceIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceIDHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *TraceIDHandler) WithGroup(name string) slog.Handler {
	return &TraceIDHandler{Handler: h.Handler.WithGroup(name)}
}
