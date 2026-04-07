// Package observability provides OpenTelemetry instrumentation for OpenTide.
// Traces, metrics, and structured attributes for gateway, skills, and approval.
package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer is the global OpenTide tracer.
var Tracer trace.Tracer

// Meter is the global OpenTide meter.
var Meter metric.Meter

// Metrics holds pre-registered metric instruments.
var Metrics *MetricSet

// MetricSet contains all registered metrics.
type MetricSet struct {
	MessageCount    metric.Int64Counter
	MessageLatency  metric.Float64Histogram
	SkillInvocations metric.Int64Counter
	SkillLatency     metric.Float64Histogram
	SkillErrors      metric.Int64Counter
	ApprovalRequests metric.Int64Counter
	ApprovalDenials  metric.Int64Counter
	RateLimitHits    metric.Int64Counter
	ActiveSessions   metric.Int64UpDownCounter
}

// Init sets up OpenTelemetry tracing and metrics.
// In production, replace stdout exporter with OTLP exporter.
func Init(ctx context.Context, serviceName string) (func(), error) {
	// Trace exporter (stdout for dev, replace with OTLP for prod)
	traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	res := newResource(serviceName)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	Tracer = tp.Tracer("opentide")

	// Metric provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	Meter = mp.Meter("opentide")

	// Register metrics
	Metrics, err = registerMetrics(Meter)
	if err != nil {
		return nil, err
	}

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tp.Shutdown(ctx)
		mp.Shutdown(ctx)
	}

	return shutdown, nil
}

func registerMetrics(meter metric.Meter) (*MetricSet, error) {
	ms := &MetricSet{}
	var err error

	ms.MessageCount, err = meter.Int64Counter("opentide.messages.count",
		metric.WithDescription("Total messages processed"))
	if err != nil {
		return nil, err
	}

	ms.MessageLatency, err = meter.Float64Histogram("opentide.messages.latency",
		metric.WithDescription("Message processing latency in seconds"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	ms.SkillInvocations, err = meter.Int64Counter("opentide.skills.invocations",
		metric.WithDescription("Total skill invocations"))
	if err != nil {
		return nil, err
	}

	ms.SkillLatency, err = meter.Float64Histogram("opentide.skills.latency",
		metric.WithDescription("Skill execution latency in seconds"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	ms.SkillErrors, err = meter.Int64Counter("opentide.skills.errors",
		metric.WithDescription("Total skill execution errors"))
	if err != nil {
		return nil, err
	}

	ms.ApprovalRequests, err = meter.Int64Counter("opentide.approvals.requests",
		metric.WithDescription("Total approval requests"))
	if err != nil {
		return nil, err
	}

	ms.ApprovalDenials, err = meter.Int64Counter("opentide.approvals.denials",
		metric.WithDescription("Total approval denials"))
	if err != nil {
		return nil, err
	}

	ms.RateLimitHits, err = meter.Int64Counter("opentide.ratelimit.hits",
		metric.WithDescription("Total rate limit hits"))
	if err != nil {
		return nil, err
	}

	ms.ActiveSessions, err = meter.Int64UpDownCounter("opentide.sessions.active",
		metric.WithDescription("Currently active sessions"))
	if err != nil {
		return nil, err
	}

	return ms, nil
}

// Common attribute keys for consistent tracing.
var (
	AttrUserID    = attribute.Key("user.id")
	AttrChannelID = attribute.Key("channel.id")
	AttrPlatform  = attribute.Key("platform")
	AttrProvider  = attribute.Key("provider")
	AttrModel     = attribute.Key("model")
	AttrSkillName = attribute.Key("skill.name")
	AttrSkillVer  = attribute.Key("skill.version")
)

// newResource creates an OpenTelemetry resource for the service.
func newResource(serviceName string) *resource.Resource {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("0.1.0"),
		),
	)
	return r
}
