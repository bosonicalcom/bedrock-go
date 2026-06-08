package kafkax

import "github.com/twmb/franz-go/pkg/kgo"

type ConsumerRegistrar interface {
	Register(topic string, handler Handler, opts ...ConsumerRegistrarOption)
}

type consumerRegistrarOptions struct {
	groupName           string
	interceptors        []ConsumerInterceptor
	enableSeqProcessing bool
	offset              kgo.Offset
}

func newDefaultConsumerRegistrarOptions() *consumerRegistrarOptions {
	return &consumerRegistrarOptions{
		groupName:           "",
		interceptors:        nil,
		enableSeqProcessing: false,
		offset:              kgo.NewOffset().AtStart(),
	}
}

type ConsumerRegistrarOption func(options *consumerRegistrarOptions)

// WithConsumerGroup sets the consumer group name of a consumer allocated by [ConsumerRegistrar].
func WithConsumerGroup(name string) ConsumerRegistrarOption {
	return func(options *consumerRegistrarOptions) {
		options.groupName = name
	}
}

// WithConsumerInterceptors sets the interceptors of a consumer allocated by [ConsumerRegistrar].
func WithConsumerInterceptors(in ...ConsumerInterceptor) ConsumerRegistrarOption {
	return func(options *consumerRegistrarOptions) {
		if options.interceptors == nil {
			options.interceptors = make([]ConsumerInterceptor, 0, len(in))
		}
		options.interceptors = append(options.interceptors, in...)
	}
}

// WithOffset sets the initial offset of a consumer allocated by [ConsumerRegistrar].
func WithOffset(off kgo.Offset) ConsumerRegistrarOption {
	return func(options *consumerRegistrarOptions) {
		options.offset = off
	}
}

// EnableSequentialProcessing enables sequential processing of messages for a consumer allocated by [ConsumerRegistrar].
//
// Sequential processing guarantees per-partition ordering.
// Records are processed and marked one at a time — if record N fails, processing stops and no further records in the batch
// are marked. The next poll cycle retries from the last committed
// offset (which may include already-processed records if the
// auto-commit interval hadn't flushed yet).
//
// Handlers should still be idempotent to tolerate redelivery
// after crash between mark and commit.
//
// WARNING: sequential processing is unsuitable for handlers with
// high latency (e.g., LLM inference, external API calls). A
// partition with 100 pending records at 10s each takes ~16 minutes
// to drain. During this time, consumer lag grows, downstream
// timeouts may cascade, and Kafka may trigger a rebalance if
// processing exceeds session.timeout.ms (default 45s in franz-go).
// For slow handlers, use concurrent mode with idempotent/dedup
// guards instead.
func EnableSequentialProcessing() ConsumerRegistrarOption {
	return func(options *consumerRegistrarOptions) {
		options.enableSeqProcessing = true
	}
}
