package syserr_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/bosonicalcom/bedrock-go/syserr"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		code    syserr.Code
		message string
	}{
		{name: "not found", code: syserr.CodeNotFound, message: "not found"},
		{name: "internal", code: syserr.CodeInternal, message: "something broke"},
		{name: "cancelled", code: syserr.CodeCancelled, message: "cancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := syserr.New(tt.code, tt.message)
			require.NotNil(t, err)
			assert.Equal(t, tt.code, err.Code)
			assert.Equal(t, tt.message, err.Message)
			assert.Empty(t, err.Details)
			assert.Empty(t, err.Causes)
		})
	}
}

func TestNew_WithDetails(t *testing.T) {
	info := syserr.ErrorInfo{Reason: "REASON", Domain: "example.com"}
	debug := syserr.DebugInfo{Detail: "internal", StackEntries: []string{"a", "b"}}

	err := syserr.New(syserr.CodeInternal, "oops", syserr.WithDetails(info, debug))

	require.Len(t, err.Details, 2)
	assert.Equal(t, info, err.Details[0])
	assert.Equal(t, debug, err.Details[1])
}

func TestNew_WithCauses(t *testing.T) {
	cause1 := errors.New("cause1")
	cause2 := errors.New("cause2")

	err := syserr.New(syserr.CodeInternal, "wrapped", syserr.WithCauses(cause1, cause2))

	require.Len(t, err.Causes, 2)
	unwrapped := err.Unwrap()
	require.Error(t, unwrapped)
	require.ErrorIs(t, unwrapped, cause1)
	require.ErrorIs(t, unwrapped, cause2)
}

func TestError_Unwrap_NoCauses(t *testing.T) {
	assert.NoError(t, syserr.New(syserr.CodeUnknown, "bare").Unwrap())
}

func TestError_StringMethods(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{name: "simple", message: "simple message"},
		{name: "empty", message: ""},
		{name: "with spaces", message: "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := syserr.New(syserr.CodeCancelled, tt.message)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, tt.message, err.String())
		})
	}
}

func TestError_PublicAndPrivateDetails(t *testing.T) {
	pub1 := syserr.ErrorInfo{Reason: "R", Domain: "d"}
	pub2 := syserr.RetryInfo{RetryDelay: time.Second}
	pub3 := syserr.LocalizedMessage{Locale: language.English, Message: "hello"}
	priv := syserr.DebugInfo{Detail: "secret"}

	err := syserr.New(syserr.CodeInternal, "msg",
		syserr.WithDetails(pub1, priv, pub2, pub3),
	)

	t.Run("public excludes DebugInfo", func(t *testing.T) {
		public := err.PublicDetails()
		assert.Len(t, public, 3)
		assert.Contains(t, public, pub1)
		assert.Contains(t, public, pub2)
		assert.Contains(t, public, pub3)
		assert.NotContains(t, public, priv)
	})

	t.Run("private contains only DebugInfo", func(t *testing.T) {
		private := err.PrivateDetails()
		assert.Len(t, private, 1)
		assert.Contains(t, private, priv)
	})
}

func TestDetailType(t *testing.T) {
	tests := []struct {
		name     string
		detail   syserr.Detail
		wantType string
	}{
		{name: "ErrorInfo", detail: syserr.ErrorInfo{}, wantType: syserr.ErrorInfoType},
		{name: "DebugInfo", detail: syserr.DebugInfo{}, wantType: syserr.DebugInfoType},
		{name: "LocalizedMessage", detail: syserr.LocalizedMessage{}, wantType: syserr.LocalizedMessageType},
		{name: "BadRequest", detail: syserr.BadRequest{}, wantType: syserr.BadRequestType},
		{name: "PreconditionFailure", detail: syserr.PreconditionFailure{}, wantType: syserr.PreconditionFailureType},
		{name: "QuotaFailure", detail: syserr.QuotaFailure{}, wantType: syserr.QuotaFailureType},
		{name: "RetryInfo", detail: syserr.RetryInfo{}, wantType: syserr.RetryInfoType},
		{name: "ResourceInfo", detail: syserr.ResourceInfo{}, wantType: syserr.ResourceInfoType},
		{name: "RequestInfo", detail: syserr.RequestInfo{}, wantType: syserr.RequestInfoType},
		{name: "Help", detail: syserr.Help{}, wantType: syserr.HelpType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantType, tt.detail.DetailType())
		})
	}
}
