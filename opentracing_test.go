package opentracing

import (
	"context"
	"fmt"
	"testing"

	"github.com/opentracing/opentracing-go/mocktracer"
	"go.unistack.org/micro/v3/metadata"
	"go.unistack.org/micro/v3/tracer"
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
}

func TestTraceTags(t *testing.T) {
	md := metadata.New(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = metadata.NewIncomingContext(ctx, md)

	mtr := mocktracer.New()
	tr := NewTracer(Tracer(mtr))
	if err := tr.Init(); err != nil {
		t.Fatal(err)
	}

	var sp tracer.Span

	ctx, sp = tr.Start(ctx, "test", tracer.WithSpanLabels("key", "val", "odd"))
	sp.Finish(tracer.WithSpanLabels("xkey", "xval"))
	_ = ctx
	msp := mtr.FinishedSpans()[0]

	if "val" != fmt.Sprintf("%v", msp.Tags()["key"]) {
		t.Fatal("mock span invalid")
	}

	if "xval" != fmt.Sprintf("%v", msp.Tags()["xkey"]) {
		t.Fatalf("mock span invalid %#+v", msp)
	}
}
