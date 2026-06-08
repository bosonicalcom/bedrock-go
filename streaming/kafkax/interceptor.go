package kafkax

import (
	"context"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

// - Consumer

type ConsumerInterceptor func(next Handler) Handler

// ConsumerInterceptorChain applies interceptors in order (first = outermost)
func ConsumerInterceptorChain(h Handler, interceptors ...ConsumerInterceptor) Handler {
	if len(interceptors) == 0 {
		return h
	}
	// Apply in reverse so the first interceptor is outermost
	for i := len(interceptors) - 1; i >= 0; i-- {
		h = interceptors[i](h)
	}
	return h
}

// -- Logger

func LogConsumerInterceptor(logger *slog.Logger) ConsumerInterceptor {
	return func(next Handler) Handler {
		return func(ctx context.Context, record *kgo.Record) error {
			logger.InfoContext(ctx, "processing kafka message")
			return next(ctx, record)
		}
	}
}
