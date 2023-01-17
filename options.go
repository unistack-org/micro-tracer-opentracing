package opentracing

import (
	"github.com/opentracing/opentracing-go"
	"go.unistack.org/micro/v3/tracer"
)

type tracerKey struct{}

func Tracer(ot opentracing.Tracer) tracer.Option {
	return tracer.SetOption(tracerKey{}, ot)
}
