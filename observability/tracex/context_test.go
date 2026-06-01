package tracex_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bosonicalcom/bedrock-go/observability/tracex"
)

func TestTraceIDContext(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() context.Context
		wantID    string
		wantFound bool
	}{
		{
			name:      "round-trip",
			setup:     func() context.Context { return tracex.WithTraceID(context.Background(), "abc-123") },
			wantID:    "abc-123",
			wantFound: true,
		},
		{
			name:      "missing on empty context",
			setup:     func() context.Context { return context.Background() },
			wantFound: false,
		},
		{
			name: "overwrite keeps last value",
			setup: func() context.Context {
				ctx := tracex.WithTraceID(context.Background(), "first")
				return tracex.WithTraceID(ctx, "second")
			},
			wantID:    "second",
			wantFound: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tracex.TraceIDFromContext(tt.setup())
			assert.Equal(t, tt.wantFound, ok)
			if tt.wantFound {
				assert.Equal(t, tt.wantID, got)
			}
		})
	}
}
