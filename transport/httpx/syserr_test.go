package httpx_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/bosonicalcom/bedrock-go/syserr"
	"github.com/bosonicalcom/bedrock-go/transport/httpx"
)

func makeResponse(t *testing.T, env httpx.ErrorEnvelope) *http.Response {
	t.Helper()
	body, err := json.Marshal(env)
	require.NoError(t, err)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func TestParseSystemError(t *testing.T) {
	tests := []struct {
		name        string
		env         httpx.ErrorEnvelope
		wantCode    syserr.Code
		wantMessage string
		wantDetails []syserr.Detail
	}{
		{
			name: "not found / no details",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusNotFound,
				Message:    "resource not found",
				StatusCode: string(syserr.CodeNotFound),
			}},
			wantCode:    syserr.CodeNotFound,
			wantMessage: "resource not found",
		},
		{
			name: "internal error",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusInternalServerError,
				Message:    "something went wrong",
				StatusCode: string(syserr.CodeInternal),
			}},
			wantCode:    syserr.CodeInternal,
			wantMessage: "something went wrong",
		},
		{
			name: "error info detail",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusForbidden,
				Message:    "not allowed",
				StatusCode: string(syserr.CodePermissionDenied),
				Details: []any{
					httpx.ErrorInfoResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.ErrorInfoType},
						Reason:         "USER_BANNED",
						Domain:         "example.com",
						Metadata:       map[string]string{"key": "val"},
					},
				},
			}},
			wantCode:    syserr.CodePermissionDenied,
			wantMessage: "not allowed",
			wantDetails: []syserr.Detail{
				syserr.ErrorInfo{Reason: "USER_BANNED", Domain: "example.com", Metadata: map[string]string{"key": "val"}},
			},
		},
		{
			name: "bad request detail",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusBadRequest,
				Message:    "invalid input",
				StatusCode: string(syserr.CodeInvalidArgument),
				Details: []any{
					httpx.BadRequestResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.BadRequestType},
						Violations: []httpx.FieldViolationResponse{
							{Field: "email", Description: "invalid format", Reason: "INVALID_EMAIL"},
						},
					},
				},
			}},
			wantCode:    syserr.CodeInvalidArgument,
			wantMessage: "invalid input",
			wantDetails: []syserr.Detail{
				syserr.BadRequest{Violations: []syserr.FieldViolation{
					{Field: "email", Description: "invalid format", Reason: "INVALID_EMAIL"},
				}},
			},
		},
		{
			name: "bad request with localized message",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusBadRequest,
				Message:    "invalid input",
				StatusCode: string(syserr.CodeInvalidArgument),
				Details: []any{
					httpx.BadRequestResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.BadRequestType},
						Violations: []httpx.FieldViolationResponse{
							{
								Field:       "name",
								Description: "too short",
								LocalizedMessage: httpx.LocalizedMessageResponse{
									DetailResponse: httpx.DetailResponse{TypeUrl: syserr.LocalizedMessageType},
									Locale:         "es",
									Message:        "demasiado corto",
								},
							},
						},
					},
				},
			}},
			wantCode:    syserr.CodeInvalidArgument,
			wantMessage: "invalid input",
			wantDetails: []syserr.Detail{
				syserr.BadRequest{Violations: []syserr.FieldViolation{
					{Field: "name", Description: "too short", LocalizedMessage: syserr.LocalizedMessage{
						Locale:  language.Make("es"),
						Message: "demasiado corto",
					}},
				}},
			},
		},
		{
			name: "precondition failure detail",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusPreconditionFailed,
				Message:    "precondition failed",
				StatusCode: string(syserr.CodeFailedPrecondition),
				Details: []any{
					httpx.PreconditionFailureResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.PreconditionFailureType},
						Violations: []httpx.PreconditionViolationResponse{
							{Type: "ACCOUNT_SUSPENDED", Subject: "user/123", Description: "account is suspended"},
						},
					},
				},
			}},
			wantCode:    syserr.CodeFailedPrecondition,
			wantMessage: "precondition failed",
			wantDetails: []syserr.Detail{
				syserr.PreconditionFailure{Violations: []syserr.PreconditionViolation{
					{Type: "ACCOUNT_SUSPENDED", Subject: "user/123", Description: "account is suspended"},
				}},
			},
		},
		{
			name: "quota failure detail",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusTooManyRequests,
				Message:    "quota exceeded",
				StatusCode: string(syserr.CodeResourceExhausted),
				Details: []any{
					httpx.QuotaFailureResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.QuotaFailureType},
						Violations: []httpx.QuotaViolationResponse{
							{Subject: "projects/my-project", Description: "daily limit reached"},
						},
					},
				},
			}},
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
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusServiceUnavailable,
				Message:    "try again later",
				StatusCode: string(syserr.CodeUnavailable),
				Details: []any{
					httpx.RetryInfoResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.RetryInfoType},
						RetryDelay:     "5s",
					},
				},
			}},
			wantCode:    syserr.CodeUnavailable,
			wantMessage: "try again later",
			wantDetails: []syserr.Detail{
				syserr.RetryInfo{RetryDelay: 5 * time.Second},
			},
		},
		{
			name: "resource info detail",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusNotFound,
				Message:    "resource not found",
				StatusCode: string(syserr.CodeNotFound),
				Details: []any{
					httpx.ResourceInfoResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.ResourceInfoType},
						ResourceType:   "Widget",
						ResourceName:   "widgets/42",
						Owner:          "user/1",
						Description:    "widget does not exist",
					},
				},
			}},
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
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusInternalServerError,
				Message:    "internal error",
				StatusCode: string(syserr.CodeInternal),
				Details: []any{
					httpx.RequestInfoResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.RequestInfoType},
						RequestID:      "req-abc-123",
					},
				},
			}},
			wantCode:    syserr.CodeInternal,
			wantMessage: "internal error",
			wantDetails: []syserr.Detail{
				syserr.RequestInfo{RequestID: "req-abc-123"},
			},
		},
		{
			name: "help detail",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusBadRequest,
				Message:    "see docs",
				StatusCode: string(syserr.CodeInvalidArgument),
				Details: []any{
					httpx.HelpResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.HelpType},
						Links: []httpx.HelpLinkResponse{
							{Description: "API docs", URL: "https://example.com/docs"},
						},
					},
				},
			}},
			wantCode:    syserr.CodeInvalidArgument,
			wantMessage: "see docs",
			wantDetails: []syserr.Detail{
				syserr.Help{Links: []syserr.HelpLink{
					{Description: "API docs", URL: "https://example.com/docs"},
				}},
			},
		},
		{
			name: "localized message detail",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusNotFound,
				Message:    "not found",
				StatusCode: string(syserr.CodeNotFound),
				Details: []any{
					httpx.LocalizedMessageResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.LocalizedMessageType},
						Locale:         "fr",
						Message:        "ressource introuvable",
					},
				},
			}},
			wantCode:    syserr.CodeNotFound,
			wantMessage: "not found",
			wantDetails: []syserr.Detail{
				syserr.LocalizedMessage{Locale: language.Make("fr"), Message: "ressource introuvable"},
			},
		},
		{
			name: "debug info is dropped",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusInternalServerError,
				Message:    "internal error",
				StatusCode: string(syserr.CodeInternal),
				Details: []any{
					// DebugInfo is intentionally not a public detail response type in httpx;
					// simulate a raw JSON object with the debug type URL.
					map[string]any{
						"@type":  syserr.DebugInfoType,
						"detail": "secret stack trace",
					},
				},
			}},
			wantCode:    syserr.CodeInternal,
			wantMessage: "internal error",
			wantDetails: nil,
		},
		{
			name: "multiple details",
			env: httpx.ErrorEnvelope{Error: httpx.ErrorResponse{
				Code:       http.StatusBadRequest,
				Message:    "bad request",
				StatusCode: string(syserr.CodeInvalidArgument),
				Details: []any{
					httpx.ErrorInfoResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.ErrorInfoType},
						Reason:         "FIELD_INVALID",
						Domain:         "example.com",
					},
					httpx.RequestInfoResponse{
						DetailResponse: httpx.DetailResponse{TypeUrl: syserr.RequestInfoType},
						RequestID:      "req-xyz",
					},
				},
			}},
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
			resp := makeResponse(t, tt.env)
			got, err := httpx.ParseSystemError(resp)
			require.NoError(t, err)
			require.NotNil(t, got)

			assert.Equal(t, tt.wantCode, got.Code)
			assert.Equal(t, tt.wantMessage, got.Message)
			assert.Equal(t, tt.wantDetails, got.Details)
		})
	}
}

func TestParseSystemError_Errors(t *testing.T) {
	tests := []struct {
		name string
		resp *http.Response
	}{
		{
			name: "non-JSON content type",
			resp: &http.Response{
				Header: http.Header{"Content-Type": []string{"text/plain"}},
				Body:   io.NopCloser(bytes.NewReader([]byte("error"))),
			},
		},
		{
			name: "nil body",
			resp: &http.Response{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nil,
			},
		},
		{
			name: "malformed JSON",
			resp: &http.Response{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   io.NopCloser(bytes.NewReader([]byte(`{not valid json`))),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := httpx.ParseSystemError(tt.resp)
			assert.Error(t, err)
			assert.Nil(t, got)
		})
	}
}
