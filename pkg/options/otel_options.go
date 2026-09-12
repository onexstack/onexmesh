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
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	otellog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	emptyexporter "github.com/onexstack/onexmesh/pkg/otel/empty"
	"github.com/onexstack/onexmesh/pkg/otelslog"
)

var _ IOptions = (*OTelOptions)(nil)

// OutputMode represents the output mode for OpenTelemetry data.
type OutputMode string

const (
	// OutputModeOTLP sends all three signals to an OTel collector over OTLP.
	OutputModeOTLP OutputMode = "otel"
	// OutputModeFile writes all three signals to local files.
	OutputModeFile OutputMode = "file"
	// OutputModeConsole writes all three signals to standard output.
	OutputModeConsole OutputMode = "console"
	// OutputModeClassic uses traditional logging/metrics only (no OTel trace).
	OutputModeClassic OutputMode = "classic"
	// OutputModeHybrid routes each signal independently:
	// log->stdout, metric->prometheus, trace->otel.
	OutputModeHybrid OutputMode = "hybrid"
)

// String implements the Stringer interface.
func (o OutputMode) String() string { return string(o) }

// IsValid reports whether the output mode is a known value.
func (o OutputMode) IsValid() bool {
	switch o {
	case OutputModeConsole, OutputModeFile, OutputModeOTLP, OutputModeClassic, OutputModeHybrid:
		return true
	default:
		return false
	}
}

// outputModeFlag implements pflag.Value for OutputMode.
type outputModeFlag OutputMode

func (f *outputModeFlag) String() string { return string(*f) }
func (f *outputModeFlag) Type() string   { return "string" }
func (f *outputModeFlag) Set(s string) error {
	mode := OutputMode(s)
	if !mode.IsValid() {
		return fmt.Errorf("invalid output mode: %s, valid options: otel, file, console, classic, hybrid", s)
	}
	*f = outputModeFlag(mode)
	return nil
}

// Provider wraps OpenTelemetry providers with shutdown capability.
type Provider interface {
	Shutdown(context.Context) error
}

// OTelProviders holds all OpenTelemetry providers.
type OTelProviders struct {
	tracer *trace.TracerProvider
	meter  *metric.MeterProvider
	logger *otellog.LoggerProvider
}

// OTelOptions configures OpenTelemetry trace, metric and log.
type OTelOptions struct {
	// Connection settings.
	Endpoint string
	Insecure bool

	// Service identification.
	ServiceName       string
	ServiceVersion    string
	ServiceInstanceID string
	Environment       string

	// Behavior settings.
	SamplingRatio float64
	WithResource  bool

	// Output configuration.
	OutputMode OutputMode
	OutputDir  string

	// Logging configuration (synced into Slog).
	Level     string
	AddSource bool

	Slog *SlogOptions

	// Internal state.
	mu        sync.RWMutex
	providers *OTelProviders
	files     []io.Closer

	resourceOnce sync.Once
	resource     *resource.Resource
}

// NewOTelOptions creates a new OTelOptions with sensible defaults.
func NewOTelOptions() *OTelOptions {
	hostname, _ := os.Hostname()
	opts := &OTelOptions{
		ServiceName:       "unknown-service",
		ServiceVersion:    "v0.0.0",
		ServiceInstanceID: hostname,
		Environment:       "development",
		Endpoint:          "localhost:4317",
		Insecure:          true,
		SamplingRatio:     1.0,
		OutputMode:        OutputModeClassic,
		OutputDir:         "./otel-output",
		Level:             "info",
		AddSource:         false,
		Slog:              NewSlogOptions(),
		providers:         &OTelProviders{},
		files:             make([]io.Closer, 0),
	}

	// Sync slog options.
	opts.syncSlogOptions()
	return opts
}

// syncSlogOptions synchronizes slog options with main options.
func (o *OTelOptions) syncSlogOptions() {
	if o.Slog != nil {
		o.Slog.Level = o.Level
		o.Slog.AddSource = o.AddSource
	}
}

// Validate validates the configuration.
func (o *OTelOptions) Validate() []error {
	var errs []error

	// Sync slog options before validation.
	o.syncSlogOptions()

	if o.Slog != nil {
		errs = append(errs, o.Slog.Validate()...)
	}

	if !o.OutputMode.IsValid() {
		errs = append(errs, fmt.Errorf("invalid output mode: %s", o.OutputMode))
	}

	if o.ServiceName == "" {
		errs = append(errs, fmt.Errorf("service name is required"))
	}
	if o.ServiceInstanceID == "" {
		errs = append(errs, fmt.Errorf("service instance ID is required"))
	}
	if o.OutputMode == OutputModeOTLP && o.Endpoint == "" {
		errs = append(errs, fmt.Errorf("endpoint is required for OTLP output mode"))
	}
	if o.SamplingRatio < 0 || o.SamplingRatio > 1 {
		errs = append(errs, fmt.Errorf("sampling ratio must be between 0 and 1, got: %f", o.SamplingRatio))
	}
	if o.OutputMode == OutputModeFile && o.OutputDir == "" {
		errs = append(errs, fmt.Errorf("output directory is required for file output mode"))
	}

	return errs
}

// AddFlags adds command line flags.
func (o *OTelOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.ServiceName, fullPrefix+".service-name", o.ServiceName, "Service name.")
	fs.StringVar(&o.ServiceVersion, fullPrefix+".service-version", o.ServiceVersion, "Service version.")
	fs.StringVar(&o.ServiceInstanceID, fullPrefix+".service-instance-id", o.ServiceInstanceID, "Service instance ID.")
	fs.StringVar(&o.Environment, fullPrefix+".environment", o.Environment, "Environment.")
	fs.StringVar(&o.Endpoint, fullPrefix+".endpoint", o.Endpoint, "OTLP endpoint.")
	fs.BoolVar(&o.Insecure, fullPrefix+".insecure", o.Insecure, "Use insecure connection.")
	fs.Float64Var(&o.SamplingRatio, fullPrefix+".sampling-ratio", o.SamplingRatio, "Sampling ratio (0.0-1.0).")
	fs.BoolVar(&o.WithResource, fullPrefix+".with-resource", o.WithResource, "Include system resource information.")
	fs.Var((*outputModeFlag)(&o.OutputMode), fullPrefix+".output-mode", "Output mode: otel, file, console, classic, hybrid.")
	fs.StringVar(&o.OutputDir, fullPrefix+".output-dir", o.OutputDir, "Output directory for file mode.")
	fs.StringVar(&o.Level, fullPrefix+".level", o.Level, "Log level: debug, info, warn, error.")
	fs.BoolVar(&o.AddSource, fullPrefix+".add-source", o.AddSource, "Add source code position to logs.")
}

// GetResource creates or returns the cached resource configuration for this
// options instance. The cache is per-instance (not package-level) so multiple
// OTelOptions with different service identities do not cross-contaminate.
func (o *OTelOptions) GetResource() *resource.Resource {
	o.resourceOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		attrs := []resource.Option{
			resource.WithAttributes(
				semconv.ServiceName(o.ServiceName),
				semconv.ServiceVersion(o.ServiceVersion),
				semconv.ServiceInstanceID(o.ServiceInstanceID),
				semconv.DeploymentEnvironment(o.Environment),
			),
		}

		if o.WithResource {
			attrs = append(attrs,
				resource.WithOS(),
				resource.WithProcess(),
				resource.WithContainer(),
				resource.WithHost(),
			)
		}

		var err error
		o.resource, err = resource.New(ctx, attrs...)
		if err != nil {
			o.resource = resource.Default()
		}
	})
	return o.resource
}

// createFileWriter creates and manages a file writer under OutputDir.
func (o *OTelOptions) createFileWriter(name string) (io.Writer, error) {
	if err := os.MkdirAll(o.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := filepath.Join(o.OutputDir, name+".json")
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file %s: %w", filename, err)
	}

	o.files = append(o.files, file)
	return file, nil
}

// initTraces initializes tracing.
func (o *OTelOptions) initTraces(ctx context.Context) error {
	var (
		exporter trace.SpanExporter
		err      error
	)

	switch o.OutputMode {
	case OutputModeClassic:
		exporter = emptyexporter.NewEmptyExporter()
	case OutputModeOTLP, OutputModeHybrid:
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(o.Endpoint)}
		if o.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
	case OutputModeFile:
		writer, writerErr := o.createFileWriter("traces")
		if writerErr != nil {
			return fmt.Errorf("failed to create trace writer: %w", writerErr)
		}
		exporter, err = stdouttrace.New(stdouttrace.WithWriter(writer))
	default: // OutputModeConsole and fallback
		exporter, err = stdouttrace.New(stdouttrace.WithWriter(os.Stdout))
	}

	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(o.GetResource()),
		trace.WithSampler(trace.TraceIDRatioBased(o.SamplingRatio)),
	)

	o.providers.tracer = tp
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// initMetrics initializes metrics.
func (o *OTelOptions) initMetrics(ctx context.Context) error {
	var (
		reader   metric.Reader
		exporter metric.Exporter
		err      error
	)

	switch o.OutputMode {
	case OutputModeClassic, OutputModeHybrid:
		reader, err = prometheus.New()
	case OutputModeOTLP:
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(o.Endpoint)}
		if o.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		exporter, err = otlpmetricgrpc.New(ctx, opts...)
	case OutputModeFile:
		writer, writerErr := o.createFileWriter("metrics")
		if writerErr != nil {
			return fmt.Errorf("failed to create metrics writer: %w", writerErr)
		}
		exporter, err = stdoutmetric.New(stdoutmetric.WithWriter(writer))
	default: // OutputModeConsole and fallback
		exporter, err = stdoutmetric.New(stdoutmetric.WithWriter(os.Stdout))
	}

	if err != nil {
		return fmt.Errorf("failed to create metrics exporter: %w", err)
	}

	if exporter != nil {
		reader = metric.NewPeriodicReader(exporter)
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(o.GetResource()),
	)

	o.providers.meter = mp
	otel.SetMeterProvider(mp)
	return nil
}

// initLogs initializes logging.
func (o *OTelOptions) initLogs(ctx context.Context) error {
	var (
		exporter otellog.Exporter
		err      error
	)

	switch o.OutputMode {
	case OutputModeClassic, OutputModeHybrid:
		return o.Slog.Apply()
	case OutputModeOTLP:
		opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(o.Endpoint)}
		if o.Insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		exporter, err = otlploggrpc.New(ctx, opts...)
	case OutputModeFile:
		writer, writerErr := o.createFileWriter("logs")
		if writerErr != nil {
			return fmt.Errorf("failed to create logs writer: %w", writerErr)
		}
		exporter, err = stdoutlog.New(stdoutlog.WithWriter(writer))
	default: // OutputModeConsole and fallback
		exporter, err = stdoutlog.New(stdoutlog.WithWriter(os.Stdout))
	}

	if err != nil {
		return fmt.Errorf("failed to create log exporter: %w", err)
	}

	var processor otellog.Processor
	if o.OutputMode == OutputModeConsole {
		processor = otellog.NewSimpleProcessor(exporter)
	} else {
		processor = otellog.NewBatchProcessor(exporter)
	}

	lp := otellog.NewLoggerProvider(
		otellog.WithProcessor(processor),
		otellog.WithResource(o.GetResource()),
	)

	o.providers.logger = lp
	global.SetLoggerProvider(lp)

	// Bridge slog into OTel so logs carry the same resource and context.
	logger := otelslog.NewLogger(
		o.ServiceName,
		otelslog.WithLoggerProvider(global.GetLoggerProvider()),
		otelslog.WithSource(o.AddSource),
		otelslog.WithLevelString(o.Level),
	)
	slog.SetDefault(logger)

	return nil
}

// Apply applies the configuration by initializing all three signals.
func (o *OTelOptions) Apply() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.syncSlogOptions()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := o.initTraces(ctx); err != nil {
		return fmt.Errorf("failed to initialize traces: %w", err)
	}
	if err := o.initMetrics(ctx); err != nil {
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}
	if err := o.initLogs(ctx); err != nil {
		return fmt.Errorf("failed to initialize logs: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down all providers and closes files.
func (o *OTelOptions) Shutdown(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	var errs []error

	if o.providers.tracer != nil {
		if err := o.providers.tracer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
		}
	}
	if o.providers.meter != nil {
		if err := o.providers.meter.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter shutdown: %w", err))
		}
	}
	if o.providers.logger != nil {
		if err := o.providers.logger.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("logger shutdown: %w", err))
		}
	}

	for _, file := range o.files {
		if err := file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("file close: %w", err))
		}
	}
	o.files = o.files[:0]

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// GetTracerProvider returns the tracer provider.
func (o *OTelOptions) GetTracerProvider() *trace.TracerProvider {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.providers.tracer
}

// GetMeterProvider returns the meter provider.
func (o *OTelOptions) GetMeterProvider() *metric.MeterProvider {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.providers.meter
}

// GetLoggerProvider returns the logger provider.
func (o *OTelOptions) GetLoggerProvider() *otellog.LoggerProvider {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.providers.logger
}
