package opentracing

import (
	"context"
	"sync"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"go.unistack.org/micro/v4/metadata"
)

func TestStartSpanFromIncomingContext(t *testing.T) {
	md := metadata.New(2)
	md.Set("key", "val")

	var g sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = metadata.NewIncomingContext(ctx, md)

	tracer := opentracing.GlobalTracer()
	ot := &otTracer{tracer: tracer}

	g.Add(8000)
	cherr := make(chan error)
	for i := 0; i < 8000; i++ {
		go func() {
			defer g.Done()
			_, sp := ot.startSpanFromIncomingContext(ctx, tracer, "test")
			sp.Finish()
		}()
	}

	for {
		select {
		default:
			g.Wait()
			close(cherr)
		case err, ok := <-cherr:
			if err != nil {
				t.Fatal(err)
			} else if !ok {
				return
			}
		}
	}
}
