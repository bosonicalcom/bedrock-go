package kafkax

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Handler func(ctx context.Context, record *kgo.Record) error
