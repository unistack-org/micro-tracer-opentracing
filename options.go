package opentracing

import (
	"github.com/opentracing/opentracing-go"
	"go.unistack.org/micro/v4/options"
)

type tracerKey struct{}

func Tracer(ot opentracing.Tracer) options.Option {
	return options.ContextOption(tracerKey{}, ot)
}
