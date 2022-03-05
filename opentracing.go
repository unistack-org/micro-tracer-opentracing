// Package opentracing provides wrappers for OpenTracing
package opentracing

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
	"go.unistack.org/micro/v3/metadata"
	"go.unistack.org/micro/v3/tracer"
)

var _ tracer.Tracer = &opentracingTracer{}

type opentracingTracer struct {
	opts tracer.Options
}

func (ot *opentracingTracer) Name() string {
	return ot.opts.Name
}

func (ot *opentracingTracer) Init(opts ...tracer.Option) error {
	return nil
}

func (ot *opentracingTracer) Start(ctx context.Context, name string, opts ...tracer.SpanOption) (context.Context, tracer.Span) {
	return nil, nil
}

func NewTracer(opts ...tracer.Option) *opentracingTracer {
	options := tracer.NewOptions(opts...)
	return &opentracingTracer{opts: options}
}

func spanFromContext(ctx context.Context) opentracing.Span {
	return opentracing.SpanFromContext(ctx)
}

// StartSpanFromOutgoingContext returns a new span with the given operation name and options. If a span
// is found in the context, it will be used as the parent of the resulting span.
func StartSpanFromOutgoingContext(ctx context.Context, tracer opentracing.Tracer, name string, opts ...opentracing.StartSpanOption) (context.Context, opentracing.Span, error) {
	var parentCtx opentracing.SpanContext

	md, ok := metadata.FromIncomingContext(ctx)
	// Find parent span.
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		// First try to get span within current service boundary.
		parentCtx = parentSpan.Context()
	} else if spanCtx, err := tracer.Extract(opentracing.TextMap, opentracing.TextMapCarrier(md)); err == nil && ok {
		// If there doesn't exist, try to get it from metadata(which is cross boundary)
		parentCtx = spanCtx
	}

	if parentCtx != nil {
		opts = append(opts, opentracing.ChildOf(parentCtx))
	}

	nmd := metadata.Copy(md)

	sp := tracer.StartSpan(name, opts...)
	if err := sp.Tracer().Inject(sp.Context(), opentracing.TextMap, opentracing.TextMapCarrier(nmd)); err != nil {
		return nil, nil, err
	}

	ctx = metadata.NewOutgoingContext(opentracing.ContextWithSpan(ctx, sp), nmd)

	return ctx, sp, nil
}

// StartSpanFromIncomingContext returns a new span with the given operation name and options. If a span
// is found in the context, it will be used as the parent of the resulting span.
func StartSpanFromIncomingContext(ctx context.Context, tracer opentracing.Tracer, name string, opts ...opentracing.StartSpanOption) (context.Context, opentracing.Span, error) {
	var parentCtx opentracing.SpanContext

	// Find parent span.
	md, ok := metadata.FromIncomingContext(ctx)
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		// First try to get span within current service boundary.
		parentCtx = parentSpan.Context()
	} else if spanCtx, err := tracer.Extract(opentracing.TextMap, opentracing.TextMapCarrier(md)); err == nil && ok {
		// If there doesn't exist, try to get it from metadata(which is cross boundary)
		parentCtx = spanCtx
	}

	if parentCtx != nil {
		opts = append(opts, opentracing.ChildOf(parentCtx))
	}

	var nmd metadata.Metadata
	if ok {
		nmd = metadata.New(len(md))
	} else {
		nmd = metadata.New(0)
	}

	sp := tracer.StartSpan(name, opts...)
	if err := sp.Tracer().Inject(sp.Context(), opentracing.TextMap, opentracing.TextMapCarrier(nmd)); err != nil {
		return nil, nil, err
	}

	for k, v := range md {
		nmd.Set(k, v)
	}

	ctx = metadata.NewIncomingContext(opentracing.ContextWithSpan(ctx, sp), nmd)

	return ctx, sp, nil
}
