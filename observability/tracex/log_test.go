package tracex_test

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/bosonicalcom/bedrock-go/observability/logx"
	"github.com/bosonicalcom/bedrock-go/observability/tracex"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
)

type logEntry struct {
	Time    time.Time `json:"time"`
	Message string    `json:"msg"`
	Level   string    `json:"level"`
	TraceID string    `json:"trace_id"`
}

func TestNewInterceptorHandler(t *testing.T) {
	// arrange
	buf := &bytes.Buffer{}
	var handler slog.Handler
	handler = slog.NewJSONHandler(buf, nil)
	handler = logx.NewInterceptorHandler(handler, tracex.LoggerInterceptorFunc)
	logger := slog.New(handler)

	// act
	traceID := "trace-123"
	ctx := tracex.WithTraceID(t.Context(), traceID)
	logger.InfoContext(ctx, "test log message")

	// assert
	out := logEntry{}
	err := sonic.Unmarshal(buf.Bytes(), &out)
	t.Logf("buf: %s", buf.String())
	assert.NoError(t, err)
	assert.Equal(t, "test log message", out.Message)
	assert.Equal(t, "INFO", out.Level)
	assert.Equal(t, traceID, out.TraceID)
}
