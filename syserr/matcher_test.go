package syserr_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bosonicalcom/bedrock-go/syserr"
)

func TestIs(t *testing.T) {
	notFound := syserr.New(syserr.CodeNotFound, "not found")

	tests := []struct {
		name string
		err  error
		code syserr.Code
		want bool
	}{
		{name: "matching code", err: notFound, code: syserr.CodeNotFound, want: true},
		{name: "wrong code", err: notFound, code: syserr.CodeInternal, want: false},
		{name: "nil error", err: nil, code: syserr.CodeNotFound, want: false},
		{name: "plain error", err: errors.New("plain"), code: syserr.CodeNotFound, want: false},
		{
			name: "checks top-level code not cause",
			err:  syserr.New(syserr.CodeInternal, "outer", syserr.WithCauses(notFound)),
			code: syserr.CodeNotFound,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, syserr.Is(tt.err, tt.code))
		})
	}
}

func TestIsDetailType(t *testing.T) {
	withDebug := syserr.New(syserr.CodeInternal, "msg",
		syserr.WithDetails(syserr.DebugInfo{Detail: "x"}),
	)

	t.Run("found matching type", func(t *testing.T) {
		assert.True(t, syserr.IsDetailType[syserr.DebugInfo](withDebug))
	})
	t.Run("not found different type", func(t *testing.T) {
		assert.False(t, syserr.IsDetailType[syserr.ErrorInfo](withDebug))
	})
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, syserr.IsDetailType[syserr.DebugInfo](nil))
	})
	t.Run("plain error", func(t *testing.T) {
		assert.False(t, syserr.IsDetailType[syserr.DebugInfo](errors.New("plain")))
	})
}

func TestIsPublicDetail(t *testing.T) {
	tests := []struct {
		name   string
		detail syserr.Detail
		want   bool
	}{
		{name: "ErrorInfo", detail: syserr.ErrorInfo{}, want: true},
		{name: "BadRequest", detail: syserr.BadRequest{}, want: true},
		{name: "RequestInfo", detail: syserr.RequestInfo{}, want: true},
		{name: "Help", detail: syserr.Help{}, want: true},
		{name: "LocalizedMessage", detail: syserr.LocalizedMessage{}, want: true},
		{name: "PreconditionFailure", detail: syserr.PreconditionFailure{}, want: true},
		{name: "QuotaFailure", detail: syserr.QuotaFailure{}, want: true},
		{name: "RetryInfo", detail: syserr.RetryInfo{}, want: true},
		{name: "ResourceInfo", detail: syserr.ResourceInfo{}, want: true},
		{name: "DebugInfo is private", detail: syserr.DebugInfo{}, want: false},
		{name: "nil is private", detail: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, syserr.IsPublicDetail(tt.detail))
		})
	}
}
