package opentracing

import (
	"context"
	"errors"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"go.unistack.org/micro/v3/metadata"
	"go.unistack.org/micro/v3/tracer"
)

var _ tracer.Tracer = &otTracer{}

type otTracer struct {
	opts   tracer.Options
	tracer opentracing.Tracer
}

func (ot *otTracer) Name() string {
	return ot.opts.Name
}

func (ot *otTracer) Flush(ctx context.Context) error {
	return nil
}

func (ot *otTracer) Init(opts ...tracer.Option) error {
	for _, o := range opts {
		o(&ot.opts)
	}

	if tr, ok := ot.opts.Context.Value(tracerKey{}).(opentracing.Tracer); ok {
		ot.tracer = tr
	} else {
		return errors.New("Tracer option missing")
	}

	return nil
}

func (ot *otTracer) Start(ctx context.Context, name string, opts ...tracer.SpanOption) (context.Context, tracer.Span) {
	options := tracer.NewSpanOptions(opts...)
	var span opentracing.Span
	switch options.Kind {
	case tracer.SpanKindInternal, tracer.SpanKindUnspecified:
		ctx, span = ot.startSpanFromContext(ctx, name)
	case tracer.SpanKindClient, tracer.SpanKindProducer:
		ctx, span = ot.startSpanFromOutgoingContext(ctx, name)
	case tracer.SpanKindServer, tracer.SpanKindConsumer:
		ctx, span = ot.startSpanFromIncomingContext(ctx, ot.tracer, name)
	}
	return ctx, &otSpan{span: span, opts: options}
}

type otSpan struct {
	span      opentracing.Span
	opts      tracer.SpanOptions
	status    tracer.SpanStatus
	statusMsg string
}

func (os *otSpan) SetStatus(st tracer.SpanStatus, msg string) {
	switch st {
	case tracer.SpanStatusError:
		os.span.SetTag("error", true)
	}
	os.status = st
	os.statusMsg = msg
}

func (os *otSpan) Status() (tracer.SpanStatus, string) {
	return os.status, os.statusMsg
}

func (os *otSpan) Tracer() tracer.Tracer {
	return &otTracer{tracer: os.span.Tracer()}
}

func (os *otSpan) Finish(opts ...tracer.SpanOption) {
	if len(os.opts.Labels) > 0 {
		os.span.LogKV(os.opts.Labels...)
	}
	os.span.Finish()
}

func (os *otSpan) AddEvent(name string, opts ...tracer.EventOption) {
	os.span.LogFields(log.Event(name))
}

func (os *otSpan) Context() context.Context {
	return opentracing.ContextWithSpan(context.Background(), os.span)
}

func (os *otSpan) SetName(name string) {
	os.span = os.span.SetOperationName(name)
}

func (os *otSpan) SetLabels(labels ...interface{}) {
	os.opts.Labels = labels
}

func (os *otSpan) Kind() tracer.SpanKind {
	return os.opts.Kind
}

func (os *otSpan) AddLabels(labels ...interface{}) {
	os.opts.Labels = append(os.opts.Labels, labels...)
}

func NewTracer(opts ...tracer.Option) *otTracer {
	options := tracer.NewOptions(opts...)
	return &otTracer{opts: options}
}

func spanFromContext(ctx context.Context) opentracing.Span {
	return opentracing.SpanFromContext(ctx)
}

func (ot *otTracer) startSpanFromContext(ctx context.Context, name string, opts ...opentracing.StartSpanOption) (context.Context, opentracing.Span) {
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		opts = append(opts, opentracing.ChildOf(parentSpan.Context()))
	}

	md := metadata.New(1)

	sp := ot.tracer.StartSpan(name, opts...)
	if err := sp.Tracer().Inject(sp.Context(), opentracing.TextMap, opentracing.TextMapCarrier(md)); err != nil {
		return nil, nil
	}

	ctx = opentracing.ContextWithSpan(ctx, sp)

	return ctx, sp
}

func (ot *otTracer) startSpanFromOutgoingContext(ctx context.Context, name string, opts ...opentracing.StartSpanOption) (context.Context, opentracing.Span) {
	var parentCtx opentracing.SpanContext

	md, ok := metadata.FromOutgoingContext(ctx)
	if ok && md != nil {
		if spanCtx, err := ot.tracer.Extract(opentracing.TextMap, opentracing.TextMapCarrier(md)); err == nil && ok {
			parentCtx = spanCtx
		}
	}

	if parentCtx != nil {
		opts = append(opts, opentracing.ChildOf(parentCtx))
	}

	nmd := metadata.Copy(md)

	sp := ot.tracer.StartSpan(name, opts...)
	if err := sp.Tracer().Inject(sp.Context(), opentracing.TextMap, opentracing.TextMapCarrier(nmd)); err != nil {
		return nil, nil
	}

	ctx = metadata.NewOutgoingContext(opentracing.ContextWithSpan(ctx, sp), nmd)

	return ctx, sp
}

func (ot *otTracer) startSpanFromIncomingContext(ctx context.Context, tracer opentracing.Tracer, name string, opts ...opentracing.StartSpanOption) (context.Context, opentracing.Span) {
	var parentCtx opentracing.SpanContext

	md, ok := metadata.FromIncomingContext(ctx)
	if ok && md != nil {
		if spanCtx, err := tracer.Extract(opentracing.TextMap, opentracing.TextMapCarrier(md)); err == nil {
			parentCtx = spanCtx
		}
	}

	if parentCtx != nil {
		opts = append(opts, opentracing.ChildOf(parentCtx))
	}

	nmd := metadata.Copy(md)

	sp := tracer.StartSpan(name, opts...)
	if err := sp.Tracer().Inject(sp.Context(), opentracing.TextMap, opentracing.TextMapCarrier(nmd)); err != nil {
		return nil, nil
	}

	ctx = metadata.NewIncomingContext(opentracing.ContextWithSpan(ctx, sp), nmd)

	return ctx, sp
}
