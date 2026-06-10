package kafkax

import (
	"context"
	"errors"
	"fmt"

	"github.com/bosonicalcom/bedrock-go/sysevent"
	"github.com/twmb/franz-go/pkg/kgo"
)

// EventPublisher is the implementation of [sysevent.Publisher] for Apache Kafka.
//
// Takes anonymous types (events) and converts them to kafka records through [RecordConverter] interface.
type EventPublisher struct {
	Client *kgo.Client
}

var _ sysevent.Publisher = (*EventPublisher)(nil)

// Publish implements [sysevent.Publisher].
func (e EventPublisher) Publish(ctx context.Context, events []any, opts ...sysevent.PublishOpt) error {
	options := &sysevent.PublishOptions{}
	for _, opt := range opts {
		opt(options)
	}

	records := make([]*kgo.Record, 0, len(events))
	for i := range events {
		converter, ok := events[i].(RecordConverter)
		if !ok {
			return errors.New("event does not implements kafka record converter interface, cannot publish")
		}
		record, err := converter.ToKafkaRecord()
		if err != nil {
			return fmt.Errorf("unable to convert event to kafka record: %w", err)
		}
		records = append(records, record)
	}

	if options.EnableAsync {
		// using non-cancelable context to avoid context cancellation errors.
		// Async op, so it won't block anyway.
		pubCtx := context.WithoutCancel(ctx)
		for i := range records {
			e.Client.Produce(pubCtx, records[i], nil)
		}
		return nil
	}

	return e.Client.ProduceSync(ctx, records...).FirstErr()
}
