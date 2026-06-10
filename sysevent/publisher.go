package sysevent

import (
	"context"
)

// Publisher is a component that broadcasts system events throughout the entire ecosystem.
type Publisher interface {
	// Publish broadcasts the given events to the event's subscribers.
	Publish(ctx context.Context, events []any, opts ...PublishOpt) error
}

type PublishOptions struct {
	EnableAsync bool
}

type PublishOpt func(*PublishOptions)

// EnableAsyncPublish allows [Publisher] to publish a batch of messages in asynchronous mode, making the routine call
// non-blocking I/O.
func EnableAsyncPublish() PublishOpt {
	return func(po *PublishOptions) {
		po.EnableAsync = true
	}
}
