package grpcx_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
	durationpb "google.golang.org/protobuf/types/known/durationpb"

	"github.com/bosonicalcom/bedrock-go/syserr"
	"github.com/bosonicalcom/bedrock-go/transport/grpcx"
)

// makeStatusErr builds a gRPC status error with the given proto details.
func makeStatusErr(t *testing.T, code codes.Code, msg string, details ...protoadapt.MessageV1) error {
	t.Helper()
	st := status.New(code, msg)
	if len(details) == 0 {
		return st.Err()
	}
	enriched, err := st.WithDetails(details...)
	require.NoError(t, err)
	return enriched.Err()
}

func TestParseSystemError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantCode    syserr.Code
		wantMessage string
		wantDetails []syserr.Detail
	}{
		{
			name:        "not found / no details",
			err:         makeStatusErr(t, codes.NotFound, "resource not found"),
			wantCode:    syserr.CodeNotFound,
			wantMessage: "resource not found",
		},
		{
			name:        "cancelled",
			err:         makeStatusErr(t, codes.Canceled, "cancelled by client"),
			wantCode:    syserr.CodeCancelled,
			wantMessage: "cancelled by client",
		},
		{
			name:        "unknown",
			err:         makeStatusErr(t, codes.Unknown, "unknown error"),
			wantCode:    syserr.CodeUnknown,
			wantMessage: "unknown error",
		},
		{
			name:        "invalid argument",
			err:         makeStatusErr(t, codes.InvalidArgument, "bad input"),
			wantCode:    syserr.CodeInvalidArgument,
			wantMessage: "bad input",
		},
		{
			name:        "deadline exceeded",
			err:         makeStatusErr(t, codes.DeadlineExceeded, "timed out"),
			wantCode:    syserr.CodeDeadlineExceeded,
			wantMessage: "timed out",
		},
		{
			name:        "already exists",
			err:         makeStatusErr(t, codes.AlreadyExists, "already exists"),
			wantCode:    syserr.CodeAlreadyExists,
			wantMessage: "already exists",
		},
		{
			name:        "permission denied",
			err:         makeStatusErr(t, codes.PermissionDenied, "not allowed"),
			wantCode:    syserr.CodePermissionDenied,
			wantMessage: "not allowed",
		},
		{
			name:        "unauthenticated",
			err:         makeStatusErr(t, codes.Unauthenticated, "missing credentials"),
			wantCode:    syserr.CodeUnauthenticated,
			wantMessage: "missing credentials",
		},
		{
			name:        "resource exhausted",
			err:         makeStatusErr(t, codes.ResourceExhausted, "quota exceeded"),
			wantCode:    syserr.CodeResourceExhausted,
			wantMessage: "quota exceeded",
		},
		{
			name:        "failed precondition",
			err:         makeStatusErr(t, codes.FailedPrecondition, "precondition failed"),
			wantCode:    syserr.CodeFailedPrecondition,
			wantMessage: "precondition failed",
		},
		{
			name:        "aborted",
			err:         makeStatusErr(t, codes.Aborted, "aborted"),
			wantCode:    syserr.CodeAborted,
			wantMessage: "aborted",
		},
		{
			name:        "out of range",
			err:         makeStatusErr(t, codes.OutOfRange, "out of range"),
			wantCode:    syserr.CodeOutOfRange,
			wantMessage: "out of range",
		},
		{
			name:        "unimplemented",
			err:         makeStatusErr(t, codes.Unimplemented, "not implemented"),
			wantCode:    syserr.CodeUnimplemented,
			wantMessage: "not implemented",
		},
		{
			name:        "unavailable",
			err:         makeStatusErr(t, codes.Unavailable, "service unavailable"),
			wantCode:    syserr.CodeUnavailable,
			wantMessage: "service unavailable",
		},
		{
			name:        "internal (default)",
			err:         makeStatusErr(t, codes.Internal, "something broke"),
			wantCode:    syserr.CodeInternal,
			wantMessage: "something broke",
		},
		{
			name:        "data loss",
			err:         makeStatusErr(t, codes.DataLoss, "corrupted payload"),
			wantCode:    syserr.CodeDataLoss,
			wantMessage: "corrupted payload",
		},
		{
			name: "error info detail",
			err: makeStatusErr(t, codes.PermissionDenied, "not allowed",
				&errdetails.ErrorInfo{
					Reason:   "USER_BANNED",
					Domain:   "example.com",
					Metadata: map[string]string{"key": "val"},
				},
			),
			wantCode:    syserr.CodePermissionDenied,
			wantMessage: "not allowed",
			wantDetails: []syserr.Detail{
				syserr.ErrorInfo{Reason: "USER_BANNED", Domain: "example.com", Metadata: map[string]string{"key": "val"}},
			},
		},
		{
			name: "bad request detail",
			err: makeStatusErr(t, codes.InvalidArgument, "invalid input",
				&errdetails.BadRequest{
					FieldViolations: []*errdetails.BadRequest_FieldViolation{
						{
							Field:            "email",
							Description:      "invalid format",
							Reason:           syserr.ReasonInvalidFormat,
							LocalizedMessage: &errdetails.LocalizedMessage{Locale: "es-MX", Message: "correo inválido"},
						},
						// no reason, no localized message: must decode to the zero value rather
						// than a bogus "und" locale.
						{Field: "name", Description: "required"},
					},
				},
			),
			wantCode:    syserr.CodeInvalidArgument,
			wantMessage: "invalid input",
			wantDetails: []syserr.Detail{
				syserr.BadRequest{Violations: []syserr.FieldViolation{
					{
						Field:       "email",
						Description: "invalid format",
						Reason:      syserr.ReasonInvalidFormat,
						LocalizedMessage: syserr.LocalizedMessage{
							Locale:  language.Make("es-MX"),
							Message: "correo inválido",
						},
					},
					{Field: "name", Description: "required"},
				}},
			},
		},
		{
			name: "localized message detail",
			err: makeStatusErr(t, codes.InvalidArgument, "invalid input",
				&errdetails.LocalizedMessage{Locale: "es-MX", Message: "entrada inválida"},
			),
			wantCode:    syserr.CodeInvalidArgument,
			wantMessage: "invalid input",
			wantDetails: []syserr.Detail{
				syserr.LocalizedMessage{Locale: language.Make("es-MX"), Message: "entrada inválida"},
			},
		},
		{
			name: "precondition failure detail",
			err: makeStatusErr(t, codes.FailedPrecondition, "precondition failed",
				&errdetails.PreconditionFailure{
					Violations: []*errdetails.PreconditionFailure_Violation{
						{Type: "ACCOUNT_SUSPENDED", Subject: "user/123", Description: "suspended"},
					},
				},
			),
			wantCode:    syserr.CodeFailedPrecondition,
			wantMessage: "precondition failed",
			wantDetails: []syserr.Detail{
				syserr.PreconditionFailure{Violations: []syserr.PreconditionViolation{
					{Type: "ACCOUNT_SUSPENDED", Subject: "user/123", Description: "suspended"},
				}},
			},
		},
		{
			name: "quota failure detail",
			err: makeStatusErr(t, codes.ResourceExhausted, "quota exceeded",
				&errdetails.QuotaFailure{
					Violations: []*errdetails.QuotaFailure_Violation{
						{Subject: "projects/my-project", Description: "daily limit reached"},
					},
				},
			),
			wantCode:    syserr.CodeResourceExhausted,
			wantMessage: "quota exceeded",
			wantDetails: []syserr.Detail{
				syserr.QuotaFailure{Violations: []syserr.QuotaViolation{
					{Subject: "projects/my-project", Description: "daily limit reached"},
				}},
			},
		},
		{
			name: "retry info detail",
			err: makeStatusErr(t, codes.Unavailable, "try again later",
				&errdetails.RetryInfo{RetryDelay: durationpb.New(5 * time.Second)},
			),
			wantCode:    syserr.CodeUnavailable,
			wantMessage: "try again later",
			wantDetails: []syserr.Detail{
				syserr.RetryInfo{RetryDelay: 5 * time.Second},
			},
		},
		{
			name: "resource info detail",
			err: makeStatusErr(t, codes.NotFound, "resource not found",
				&errdetails.ResourceInfo{
					ResourceType: "Widget",
					ResourceName: "widgets/42",
					Owner:        "user/1",
					Description:  "widget does not exist",
				},
			),
			wantCode:    syserr.CodeNotFound,
			wantMessage: "resource not found",
			wantDetails: []syserr.Detail{
				syserr.ResourceInfo{
					ResourceType: "Widget",
					ResourceName: "widgets/42",
					Owner:        "user/1",
					Description:  "widget does not exist",
				},
			},
		},
		{
			name: "request info detail",
			err: makeStatusErr(t, codes.Internal, "internal error",
				&errdetails.RequestInfo{RequestId: "req-abc-123", ServingData: "shard-7"},
			),
			wantCode:    syserr.CodeInternal,
			wantMessage: "internal error",
			wantDetails: []syserr.Detail{
				syserr.RequestInfo{RequestID: "req-abc-123", ServingData: "shard-7"},
			},
		},
		{
			name: "help detail",
			err: makeStatusErr(t, codes.InvalidArgument, "see docs",
				&errdetails.Help{
					Links: []*errdetails.Help_Link{
						{Description: "API docs", Url: "https://example.com/docs"},
					},
				},
			),
			wantCode:    syserr.CodeInvalidArgument,
			wantMessage: "see docs",
			wantDetails: []syserr.Detail{
				syserr.Help{Links: []syserr.HelpLink{
					{Description: "API docs", URL: "https://example.com/docs"},
				}},
			},
		},
		{
			name: "debug info detail",
			err: makeStatusErr(t, codes.Internal, "internal error",
				&errdetails.DebugInfo{
					Detail:       "unexpected nil pointer",
					StackEntries: []string{"main.go:10", "handler.go:42"},
				},
			),
			wantCode:    syserr.CodeInternal,
			wantMessage: "internal error",
			wantDetails: []syserr.Detail{
				syserr.DebugInfo{
					Detail:       "unexpected nil pointer",
					StackEntries: []string{"main.go:10", "handler.go:42"},
				},
			},
		},
		{
			name: "multiple details",
			err: makeStatusErr(t, codes.InvalidArgument, "bad request",
				&errdetails.ErrorInfo{Reason: "FIELD_INVALID", Domain: "example.com"},
				&errdetails.RequestInfo{RequestId: "req-xyz"},
			),
			wantCode:    syserr.CodeInvalidArgument,
			wantMessage: "bad request",
			wantDetails: []syserr.Detail{
				syserr.ErrorInfo{Reason: "FIELD_INVALID", Domain: "example.com"},
				syserr.RequestInfo{RequestID: "req-xyz"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := grpcx.ParseSystemError(tt.err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantCode, got.Code)
			assert.Equal(t, tt.wantMessage, got.Message)
			assert.Equal(t, tt.wantDetails, got.Details)
			// the original error is kept as a cause so errors.Is and status.Code keep working
			assert.Equal(t, []error{tt.err}, got.Causes)
			assert.Equal(t, status.Code(tt.err), status.Code(got))
		})
	}
}

func TestParseSystemError_Nil(t *testing.T) {
	assert.Nil(t, grpcx.ParseSystemError(nil))
}

func TestParseSystemError_NonStatusError(t *testing.T) {
	cause := errors.New("plain error")

	got := grpcx.ParseSystemError(cause)

	require.NotNil(t, got)
	assert.Equal(t, syserr.CodeUnknown, got.Code)
	assert.Equal(t, "plain error", got.Message)
	assert.Equal(t, []error{cause}, got.Causes)
	assert.ErrorIs(t, got, cause)
}

func TestParseSystemError_ContextErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode syserr.Code
	}{
		{name: "cancelled", err: context.Canceled, wantCode: syserr.CodeCancelled},
		{name: "deadline exceeded", err: context.DeadlineExceeded, wantCode: syserr.CodeDeadlineExceeded},
		{name: "wrapped cancelled", err: fmt.Errorf("call user service: %w", context.Canceled), wantCode: syserr.CodeCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := grpcx.ParseSystemError(tt.err)

			require.NotNil(t, got)
			assert.Equal(t, tt.wantCode, got.Code)
			assert.ErrorIs(t, got, tt.err)
		})
	}
}

// TestSystemErrorRoundTrip asserts that every public detail type survives the
// syserr -> gRPC status -> syserr trip without losing fields.
func TestSystemErrorRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   *syserr.Error
		// want is the expected decoded details; nil means "identical to in.Details".
		want []syserr.Detail
	}{
		{
			name: "error info",
			in: syserr.New(syserr.CodePermissionDenied, "not allowed",
				syserr.WithDetails(syserr.ErrorInfo{
					Reason:   "USER_BANNED",
					Domain:   syserr.GlobalDomain,
					Metadata: map[string]string{"userId": "42"},
				}),
			),
		},
		{
			name: "error info without reason is skipped",
			in: syserr.New(syserr.CodeInternal, "boom",
				syserr.WithDetails(syserr.ErrorInfo{Domain: syserr.GlobalDomain}),
			),
			want: []syserr.Detail{},
		},
		{
			name: "localized message",
			in: syserr.New(syserr.CodeInvalidArgument, "invalid input",
				syserr.WithDetails(syserr.LocalizedMessage{
					Locale:  language.Make("es-MX"),
					Message: "entrada inválida",
				}),
			),
		},
		{
			name: "bad request with reason and localized message",
			in: syserr.New(syserr.CodeInvalidArgument, "validation failed",
				syserr.WithDetails(syserr.BadRequest{Violations: []syserr.FieldViolation{
					{
						Field:       "user.email",
						Description: "must be a valid email",
						Reason:      syserr.ReasonInvalidFormat,
						LocalizedMessage: syserr.LocalizedMessage{
							Locale:  language.Make("es-MX"),
							Message: "correo inválido",
						},
					},
					// bare violation: proves the zero LocalizedMessage stays zero.
					{Field: "user.name", Description: "is required", Reason: syserr.ReasonRequiredValue},
				}}),
			),
		},
		{
			name: "precondition failure",
			in: syserr.New(syserr.CodeFailedPrecondition, "precondition failed",
				syserr.WithDetails(syserr.PreconditionFailure{Violations: []syserr.PreconditionViolation{
					{Type: "ACCOUNT_SUSPENDED", Subject: "user/123", Description: "suspended"},
				}}),
			),
		},
		{
			name: "quota failure",
			in: syserr.New(syserr.CodeResourceExhausted, "quota exceeded",
				syserr.WithDetails(syserr.QuotaFailure{Violations: []syserr.QuotaViolation{
					{Subject: "projects/my-project", Description: "daily limit reached"},
				}}),
			),
		},
		{
			name: "retry info",
			in: syserr.New(syserr.CodeUnavailable, "try again later",
				syserr.WithDetails(syserr.RetryInfo{RetryDelay: 5 * time.Second}),
			),
		},
		{
			name: "resource info",
			in: syserr.New(syserr.CodeNotFound, "resource not found",
				syserr.WithDetails(syserr.ResourceInfo{
					ResourceType: "Widget",
					ResourceName: "widgets/42",
					Owner:        "user/1",
					Description:  "widget does not exist",
				}),
			),
		},
		{
			name: "request info",
			in: syserr.New(syserr.CodeInternal, "internal error",
				syserr.WithDetails(syserr.RequestInfo{RequestID: "req-abc-123", ServingData: "shard-7"}),
			),
		},
		{
			name: "help",
			in: syserr.New(syserr.CodeInvalidArgument, "see docs",
				syserr.WithDetails(syserr.Help{Links: []syserr.HelpLink{
					{Description: "API docs", URL: "https://example.com/docs"},
				}}),
			),
		},
		{
			name: "debug info is redacted",
			in: syserr.New(syserr.CodeInternal, "internal error",
				syserr.WithDetails(syserr.DebugInfo{
					Detail:       "unexpected nil pointer",
					StackEntries: []string{"main.go:10"},
				}),
			),
			want: []syserr.Detail{},
		},
		{
			name: "all details preserve order",
			in: syserr.New(syserr.CodeInvalidArgument, "validation failed",
				syserr.WithDetails(
					syserr.ErrorInfo{Reason: syserr.ReasonValidationFailed, Domain: syserr.GlobalDomain},
					syserr.BadRequest{Violations: []syserr.FieldViolation{
						{Field: "email", Description: "invalid", Reason: syserr.ReasonInvalidFormat},
					}},
					syserr.LocalizedMessage{Locale: language.Make("es-MX"), Message: "falló la validación"},
					syserr.RequestInfo{RequestID: "req-1"},
					// dropped on the way out, so it must not appear in want
					syserr.DebugInfo{Detail: "internal"},
				),
			),
			want: []syserr.Detail{
				syserr.ErrorInfo{Reason: syserr.ReasonValidationFailed, Domain: syserr.GlobalDomain},
				syserr.BadRequest{Violations: []syserr.FieldViolation{
					{Field: "email", Description: "invalid", Reason: syserr.ReasonInvalidFormat},
				}},
				syserr.LocalizedMessage{Locale: language.Make("es-MX"), Message: "falló la validación"},
				syserr.RequestInfo{RequestID: "req-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.want
			if want == nil {
				want = tt.in.Details
			}

			got := grpcx.ParseSystemError(grpcx.StatusFromError(t.Context(), tt.in))

			require.NotNil(t, got)
			assert.Equal(t, tt.in.Code, got.Code)
			assert.Equal(t, tt.in.Message, got.Message)
			if len(want) == 0 {
				assert.Empty(t, got.Details)
				return
			}
			assert.Equal(t, want, got.Details)
		})
	}
}

// TestSystemErrorRoundTrip_Codes guards against a syserr.Code silently degrading to
// INTERNAL because it is missing from one of the code maps.
func TestSystemErrorRoundTrip_Codes(t *testing.T) {
	codesToTest := []syserr.Code{
		syserr.CodeCancelled,
		syserr.CodeUnknown,
		syserr.CodeInvalidArgument,
		syserr.CodeDeadlineExceeded,
		syserr.CodeNotFound,
		syserr.CodeAlreadyExists,
		syserr.CodePermissionDenied,
		syserr.CodeUnauthenticated,
		syserr.CodeResourceExhausted,
		syserr.CodeFailedPrecondition,
		syserr.CodeAborted,
		syserr.CodeOutOfRange,
		syserr.CodeUnimplemented,
		syserr.CodeInternal,
		syserr.CodeUnavailable,
		syserr.CodeDataLoss,
	}

	for _, code := range codesToTest {
		t.Run(string(code), func(t *testing.T) {
			in := syserr.New(code, "message")

			got := grpcx.ParseSystemError(grpcx.StatusFromError(t.Context(), in))

			require.NotNil(t, got)
			assert.Equal(t, code, got.Code)
		})
	}
}

// TestSystemErrorRoundTrip_ZeroLocale asserts an unset locale is omitted from the wire
// rather than shipped as the meaningless "und" tag.
func TestSystemErrorRoundTrip_ZeroLocale(t *testing.T) {
	in := syserr.New(syserr.CodeInvalidArgument, "validation failed",
		syserr.WithDetails(
			syserr.LocalizedMessage{Message: "no locale"},
			syserr.BadRequest{Violations: []syserr.FieldViolation{
				{Field: "email", Description: "is required", Reason: syserr.ReasonRequiredValue},
			}},
		),
	)

	wireErr := grpcx.StatusFromError(t.Context(), in)

	// inspect the wire form directly
	for _, detail := range status.Convert(wireErr).Details() {
		switch v := detail.(type) {
		case *errdetails.LocalizedMessage:
			assert.Empty(t, v.GetLocale())
		case *errdetails.BadRequest:
			require.Len(t, v.GetFieldViolations(), 1)
			assert.Nil(t, v.GetFieldViolations()[0].GetLocalizedMessage())
			assert.Equal(t, syserr.ReasonRequiredValue, v.GetFieldViolations()[0].GetReason())
		default:
			t.Fatalf("unexpected detail on the wire: %T", v)
		}
	}

	// and the decoded form
	got := grpcx.ParseSystemError(wireErr)
	require.NotNil(t, got)
	assert.Equal(t, in.Details, got.Details)
}
