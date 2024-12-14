package opentracing

import (
	"context"
	"testing"

	"github.com/opentracing/opentracing-go/mocktracer"
	"go.unistack.org/micro/v4/metadata"
	"go.unistack.org/micro/v4/tracer"
)

func TestTraceID(t *testing.T) {
	md := metadata.New(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = metadata.NewIncomingContext(ctx, md)

	tr := NewTracer(Tracer(mocktracer.New()))
	if err := tr.Init(); err != nil {
		t.Fatal(err)
	}

	var sp tracer.Span

	ctx, sp = tr.Start(ctx, "test")
	if v := sp.TraceID(); v != "43" {
		t.Fatalf("invalid span trace id %#+v", v)
	}
	if v := sp.SpanID(); v != "44" {
		t.Fatalf("invalid span span id %#+v", v)
	}
	_ = ctx
}
