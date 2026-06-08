package grpcx_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
						{Field: "email", Description: "invalid format"},
					},
				},
			),
			wantCode:    syserr.CodeInvalidArgument,
			wantMessage: "invalid input",
			wantDetails: []syserr.Detail{
				syserr.BadRequest{Violations: []syserr.FieldViolation{
					{Field: "email", Description: "invalid format"},
				}},
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
		})
	}
}

func TestParseSystemError_Nil(t *testing.T) {
	assert.Nil(t, grpcx.ParseSystemError(nil))
}

func TestParseSystemError_NonStatusError(t *testing.T) {
	assert.Nil(t, grpcx.ParseSystemError(errors.New("plain error")))
}
