package logx_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/bosonicalcom/bedrock-go/observability/logx"
)

type captureHandler struct {
	enabledVal bool
	records    []slog.Record
	handleErr  error
}

func (c *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return c.enabledVal }
func (c *captureHandler) Handle(_ context.Context, r slog.Record) error {
	c.records = append(c.records, r)
	return c.handleErr
}
func (c *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return c }
func (c *captureHandler) WithGroup(_ string) slog.Handler      { return c }

type InterceptorHandlerSuite struct {
	suite.Suite
	next *captureHandler
}

func (s *InterceptorHandlerSuite) SetupTest() {
	s.next = &captureHandler{enabledVal: true}
}

func (s *InterceptorHandlerSuite) TestEnabled_DelegatesToNext() {
	h := logx.NewInterceptorHandler(s.next)
	s.True(h.Enabled(context.Background(), slog.LevelInfo))

	s.next.enabledVal = false
	s.False(h.Enabled(context.Background(), slog.LevelInfo))
}

func (s *InterceptorHandlerSuite) TestHandle_DelegatesToNext() {
	h := logx.NewInterceptorHandler(s.next)

	var r slog.Record
	r.Message = "hello"

	s.Require().NoError(h.Handle(context.Background(), r))
	s.Require().Len(s.next.records, 1)
	s.Equal("hello", s.next.records[0].Message)
}

func (s *InterceptorHandlerSuite) TestHandle_InterceptorsRunInReverseOrder() {
	var order []int
	mkFn := func(n int) logx.HandleFunc {
		return func(_ context.Context, _ *slog.Record) error {
			order = append(order, n)
			return nil
		}
	}

	h := logx.NewInterceptorHandler(s.next, mkFn(1), mkFn(2), mkFn(3))
	s.Require().NoError(h.Handle(context.Background(), slog.Record{}))
	s.Equal([]int{3, 2, 1}, order)
}

func (s *InterceptorHandlerSuite) TestHandle_InterceptorError_ShortCircuits() {
	sentinel := errors.New("interceptor failed")
	errFn := func(_ context.Context, _ *slog.Record) error { return sentinel }

	h := logx.NewInterceptorHandler(s.next, errFn)
	err := h.Handle(context.Background(), slog.Record{})

	s.Require().ErrorIs(err, sentinel)
	s.Empty(s.next.records)
}

func (s *InterceptorHandlerSuite) TestWithAttrs_DelegatesToNext() {
	h := logx.NewInterceptorHandler(s.next)
	s.NotNil(h.WithAttrs([]slog.Attr{slog.String("k", "v")}))
}

func (s *InterceptorHandlerSuite) TestWithGroup_DelegatesToNext() {
	h := logx.NewInterceptorHandler(s.next)
	s.NotNil(h.WithGroup("grp"))
}

func TestInterceptorHandlerSuite(t *testing.T) {
	suite.Run(t, new(InterceptorHandlerSuite))
}
