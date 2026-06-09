// Package kafkaxtest provides reusable Kafka test helpers backed by testcontainers.
// Import this package in tests that need a real Kafka-compatible broker.
package kafkaxtest

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/twmb/franz-go/pkg/kgo"
)

const defaultImage = "confluentinc/confluent-local:7.5.0"

type containerOptions struct {
	image string
}

// ContainerOpt customises the Kafka container started by [NewContainer].
type ContainerOpt func(*containerOptions)

// WithImage sets the Docker image used for the container (e.g. "confluentinc/confluent-local:7.6.0").
func WithImage(image string) ContainerOpt {
	return func(o *containerOptions) { o.image = image }
}

// NewContainer starts a Kafka container, registers cleanup via tb.Cleanup,
// and returns the seed broker addresses to pass to [ControllerManagerConfig.SeedBrokerAddrs].
func NewContainer(tb testing.TB, opts ...ContainerOpt) []string {
	tb.Helper()

	o := &containerOptions{image: defaultImage}
	for _, opt := range opts {
		opt(o)
	}

	ctx := context.Background()
	ctr, err := kafka.Run(ctx, o.image, kafka.WithClusterID("test-cluster"))
	if err != nil {
		tb.Fatalf("kafkaxtest: start kafka container: %v", err)
	}
	tb.Cleanup(func() {
		if err := ctr.Terminate(ctx); err != nil {
			tb.Logf("kafkaxtest: terminate container: %v", err)
		}
	})

	brokers, err := ctr.Brokers(ctx)
	if err != nil {
		tb.Fatalf("kafkaxtest: get broker addresses: %v", err)
	}
	return brokers
}

// NewClient creates a *kgo.Client connected to addrs and registers cleanup via tb.Cleanup.
// Additional kgo.Opt values (e.g. consumer group, topics) can be passed as opts.
func NewClient(tb testing.TB, addrs []string, opts ...kgo.Opt) *kgo.Client {
	tb.Helper()

	allOpts := append([]kgo.Opt{kgo.SeedBrokers(addrs...)}, opts...)
	cl, err := kgo.NewClient(allOpts...)
	if err != nil {
		tb.Fatalf("kafkaxtest: create kgo client: %v", err)
	}
	tb.Cleanup(cl.Close)
	return cl
}
