package opentracing

import (
	"context"
	"errors"
	"fmt"

	ot "github.com/opentracing/opentracing-go"
	otlog "github.com/opentracing/opentracing-go/log"
	"go.unistack.org/micro/v3/metadata"
	"go.unistack.org/micro/v3/tracer"
	rutil "go.unistack.org/micro/v3/util/reflect"
)

var _ tracer.Tracer = &otTracer{}

type otTracer struct {
	opts   tracer.Options
	tracer ot.Tracer
}

func (t *otTracer) Name() string {
	return t.opts.Name
}

func (t *otTracer) Flush(ctx context.Context) error {
	return nil
}

func (t *otTracer) Init(opts ...tracer.Option) error {
	for _, o := range opts {
		o(&t.opts)
	}

	if tr, ok := t.opts.Context.Value(tracerKey{}).(ot.Tracer); ok {
		t.tracer = tr
	} else {
		return errors.New("Tracer option missing")
	}

	return nil
}

type spanContext interface {
	TraceID() idStringer
	SpanID() idStringer
}

func (t *otTracer) Start(ctx context.Context, name string, opts ...tracer.SpanOption) (context.Context, tracer.Span) {
	options := tracer.NewSpanOptions(opts...)
	var span ot.Span
	switch options.Kind {
	case tracer.SpanKindUnspecified:
		ctx, span = t.startSpanFromAny(ctx, name)
	case tracer.SpanKindInternal:
		ctx, span = t.startSpanFromContext(ctx, name)
	case tracer.SpanKindClient, tracer.SpanKindProducer:
		ctx, span = t.startSpanFromOutgoingContext(ctx, name)
	case tracer.SpanKindServer, tracer.SpanKindConsumer:
		ctx, span = t.startSpanFromIncomingContext(ctx, name)
	}

	sp := &otSpan{span: span, opts: options}

	spctx := span.Context()
	if v, ok := spctx.(spanContext); ok {
		sp.traceID = v.TraceID().String()
		sp.spanID = v.SpanID().String()
	} else {
		if val, err := rutil.StructFieldByName(spctx, "TraceID"); err == nil {
			sp.traceID = fmt.Sprintf("%v", val)
		}
		if val, err := rutil.StructFieldByName(spctx, "SpanID"); err == nil {
			sp.spanID = fmt.Sprintf("%v", val)
		}
	}

	return tracer.NewSpanContext(ctx, sp), sp
}

type idStringer struct {
	s string
}

func (s idStringer) String() string {
	return s.s
}

type otSpan struct {
	span      ot.Span
	spanID    string
	traceID   string
	opts      tracer.SpanOptions
	status    tracer.SpanStatus
	statusMsg string
}

func (os *otSpan) TraceID() string {
	return os.traceID
}

func (os *otSpan) SpanID() string {
	return os.spanID
}

func (os *otSpan) SetStatus(st tracer.SpanStatus, msg string) {
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
	if len(os.opts.Labels)%2 != 0 {
		os.opts.Labels = os.opts.Labels[:len(os.opts.Labels)-1]
	}
	os.opts.Labels = tracer.UniqLabels(os.opts.Labels)
	for idx := 0; idx < len(os.opts.Labels); idx += 2 {
		switch os.opts.Labels[idx] {
		case "err":
			os.status = tracer.SpanStatusError
			os.statusMsg = fmt.Sprintf("%v", os.opts.Labels[idx+1])
		case "error":
			continue
		case "X-Request-Id", "x-request-id":
			os.span.SetTag("x-request-id", os.opts.Labels[idx+1])
		case "rpc.call", "rpc.call_type", "rpc.flavor", "rpc.service", "rpc.method",
			"sdk.database", "db.statement", "db.args", "db.query", "db.method",
			"messaging.destination.name", "messaging.source.name", "messaging.operation":
			os.span.SetTag(fmt.Sprintf("%v", os.opts.Labels[idx]), os.opts.Labels[idx+1])
		default:
			os.span.LogKV(os.opts.Labels[idx], os.opts.Labels[idx+1])
		}
	}
	if os.status == tracer.SpanStatusError {
		os.span.SetTag("error", true)
		os.span.LogKV("error", os.statusMsg)
	}
	os.span.SetTag("span.kind", os.opts.Kind)
	os.span.Finish()
}

func (os *otSpan) AddEvent(name string, opts ...tracer.EventOption) {
	os.span.LogFields(otlog.Event(name))
}

func (os *otSpan) AddLogs(kv ...interface{}) {
	os.span.LogKV(kv...)
}

func (os *otSpan) Context() context.Context {
	return ot.ContextWithSpan(context.Background(), os.span)
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

func (t *otTracer) startSpanFromAny(ctx context.Context, name string, opts ...ot.StartSpanOption) (context.Context, ot.Span) {
	if tracerSpan, ok := tracer.SpanFromContext(ctx); ok && tracerSpan != nil {
		return t.startSpanFromContext(ctx, name, opts...)
	}

	if otSpan := ot.SpanFromContext(ctx); otSpan != nil {
		return t.startSpanFromContext(ctx, name, opts...)
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok && md != nil {
		return t.startSpanFromIncomingContext(ctx, name, opts...)
	}

	if md, ok := metadata.FromOutgoingContext(ctx); ok && md != nil {
		return t.startSpanFromOutgoingContext(ctx, name, opts...)
	}

	return t.startSpanFromContext(ctx, name, opts...)
}

func (t *otTracer) startSpanFromContext(ctx context.Context, name string, opts ...ot.StartSpanOption) (context.Context, ot.Span) {
	var parentSpan ot.Span
	if tracerSpan, ok := tracer.SpanFromContext(ctx); ok && tracerSpan != nil {
		if sp, ok := tracerSpan.(*otSpan); ok {
			parentSpan = sp.span
		}
	}
	if parentSpan == nil {
		if otSpan := ot.SpanFromContext(ctx); otSpan != nil {
			parentSpan = otSpan
		}
	}

	if parentSpan != nil {
		opts = append(opts, ot.ChildOf(parentSpan.Context()))
	}

	md := metadata.New(1)

	sp := t.tracer.StartSpan(name, opts...)
	if err := sp.Tracer().Inject(sp.Context(), ot.TextMap, ot.TextMapCarrier(md)); err != nil {
		return nil, nil
	}

	ctx = ot.ContextWithSpan(ctx, sp)

	return ctx, sp
}

func (t *otTracer) startSpanFromOutgoingContext(ctx context.Context, name string, opts ...ot.StartSpanOption) (context.Context, ot.Span) {
	var parentSpan ot.Span
	if tracerSpan, ok := tracer.SpanFromContext(ctx); ok && tracerSpan != nil {
		if sp, ok := tracerSpan.(*otSpan); ok {
			parentSpan = sp.span
		}
	}
	if parentSpan == nil {
		if otSpan := ot.SpanFromContext(ctx); otSpan != nil {
			parentSpan = otSpan
		}
	}

	md, ok := metadata.FromOutgoingContext(ctx)

	if parentSpan != nil {
		opts = append(opts, ot.ChildOf(parentSpan.Context()))
	} else {
		var parentCtx ot.SpanContext

		if ok && md != nil {
			if spanCtx, err := t.tracer.Extract(ot.TextMap, ot.TextMapCarrier(md)); err == nil && ok {
				parentCtx = spanCtx
			}
		}

		if parentCtx != nil {
			opts = append(opts, ot.ChildOf(parentCtx))
		}
	}

	nmd := metadata.Copy(md)

	sp := t.tracer.StartSpan(name, opts...)
	if err := sp.Tracer().Inject(sp.Context(), ot.TextMap, ot.TextMapCarrier(nmd)); err != nil {
		return nil, nil
	}

	ctx = metadata.NewOutgoingContext(ot.ContextWithSpan(ctx, sp), nmd)

	return ctx, sp
}

func (t *otTracer) startSpanFromIncomingContext(ctx context.Context, name string, opts ...ot.StartSpanOption) (context.Context, ot.Span) {
	var parentSpan ot.Span
	if tracerSpan, ok := tracer.SpanFromContext(ctx); ok && tracerSpan != nil {
		if sp, ok := tracerSpan.(*otSpan); ok {
			parentSpan = sp.span
		}
	}
	if parentSpan == nil {
		if otSpan := ot.SpanFromContext(ctx); otSpan != nil {
			parentSpan = otSpan
		}
	}

	md, ok := metadata.FromIncomingContext(ctx)

	if parentSpan != nil {
		opts = append(opts, ot.ChildOf(parentSpan.Context()))
	} else {
		var parentCtx ot.SpanContext

		if ok && md != nil {
			if spanCtx, err := t.tracer.Extract(ot.TextMap, ot.TextMapCarrier(md)); err == nil {
				parentCtx = spanCtx
			}
		}

		if parentCtx != nil {
			opts = append(opts, ot.ChildOf(parentCtx))
		}
	}

	nmd := metadata.Copy(md)

	sp := t.tracer.StartSpan(name, opts...)
	if err := sp.Tracer().Inject(sp.Context(), ot.TextMap, ot.TextMapCarrier(nmd)); err != nil {
		return nil, nil
	}

	ctx = metadata.NewIncomingContext(ot.ContextWithSpan(ctx, sp), nmd)

	return ctx, sp
}
