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
		return func(ctx context.Context, record *kgo.Record) (err error) {
			logAttrs := slog.Group("message",
				slog.String("key", string(record.Key)),
				slog.Int("val_len", len(record.Value)),
				slog.Int("total_header_items", len(record.Headers)),
				slog.Time("timestamp", record.Timestamp),
				slog.String("topic", record.Topic),
				slog.Int64("partition", int64(record.Partition)),
				slog.Int64("producer_id", record.ProducerID),
				slog.Int64("leader_epoch", int64(record.LeaderEpoch)),
				slog.Int64("offset", record.Offset),
			)
			logger.InfoContext(ctx, "processing kafka record", logAttrs)
			defer func() {
				if err != nil {
					logger.ErrorContext(ctx, "unable to process kafka record", logAttrs)
					return
				}
				logger.InfoContext(ctx, "proccessed kafka record", logAttrs)
			}()
			err = next(ctx, record)
			return
		}
	}
}
