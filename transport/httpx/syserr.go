package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
	"golang.org/x/text/language"

	"github.com/bosonicalcom/bedrock-go/syserr"
)

func getCodeFromSystemError(code syserr.Code) int {
	switch code {
	case syserr.CodeCancelled:
		return 499
	case syserr.CodeNotFound:
		return http.StatusNotFound
	case syserr.CodeInvalidArgument, syserr.CodeOutOfRange:
		return http.StatusBadRequest
	case syserr.CodeDeadlineExceeded:
		return http.StatusGatewayTimeout
	case syserr.CodeAlreadyExists:
		return http.StatusConflict
	case syserr.CodeUnauthenticated:
		return http.StatusUnauthorized
	case syserr.CodePermissionDenied:
		return http.StatusForbidden
	case syserr.CodeResourceExhausted:
		return http.StatusTooManyRequests
	case syserr.CodeFailedPrecondition:
		return http.StatusPreconditionFailed
	case syserr.CodeAborted:
		return 499
	case syserr.CodeUnimplemented:
		return 501
	case syserr.CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return 500
	}
}

func newDetailFromSystemError(err *syserr.Error) []any {
	details := make([]any, 0, len(err.Details))

	for _, detail := range err.Details {
		if !syserr.IsPublicDetail(detail) {
			continue
		}

		switch v := detail.(type) {
		case syserr.ErrorInfo:
			// optional: skip useless generic/internal-only noise
			if v.Reason == "" {
				continue
			}

			details = append(details, ErrorInfoResponse{
				DetailResponse: DetailResponse{
					TypeUrl: v.DetailType(),
				},
				Reason:   v.Reason,
				Domain:   v.Domain,
				Metadata: v.Metadata,
			})

		case syserr.LocalizedMessage:
			details = append(details, LocalizedMessageResponse{
				DetailResponse: DetailResponse{
					TypeUrl: v.DetailType(),
				},
				Locale:  v.Locale.String(),
				Message: v.Message,
			})

		case syserr.BadRequest:
			details = append(details, BadRequestResponse{
				DetailResponse: DetailResponse{
					TypeUrl: v.DetailType(),
				},
				Violations: lo.Map(v.Violations, func(item syserr.FieldViolation, _ int) FieldViolationResponse {
					resp := FieldViolationResponse{
						Field:       item.Field,
						Description: item.Description,
						Reason:      item.Reason,
					}

					if item.LocalizedMessage.Message != "" {
						resp.LocalizedMessage = LocalizedMessageResponse{
							DetailResponse: DetailResponse{
								TypeUrl: item.LocalizedMessage.DetailType(),
							},
							Locale:  item.LocalizedMessage.Locale.String(),
							Message: item.LocalizedMessage.Message,
						}
					}

					return resp
				}),
			})

		case syserr.PreconditionFailure:
			details = append(details, PreconditionFailureResponse{
				DetailResponse: DetailResponse{
					TypeUrl: v.DetailType(),
				},
				Violations: lo.Map(v.Violations, func(item syserr.PreconditionViolation, _ int) PreconditionViolationResponse {
					return PreconditionViolationResponse{
						Type:        item.Type,
						Subject:     item.Subject,
						Description: item.Description,
					}
				}),
			})

		case syserr.QuotaFailure:
			details = append(details, QuotaFailureResponse{
				DetailResponse: DetailResponse{
					TypeUrl: v.DetailType(),
				},
				Violations: lo.Map(v.Violations, func(item syserr.QuotaViolation, _ int) QuotaViolationResponse {
					return QuotaViolationResponse{
						Subject:     item.Subject,
						Description: item.Description,
					}
				}),
			})

		case syserr.RetryInfo:
			details = append(details, RetryInfoResponse{
				DetailResponse: DetailResponse{
					TypeUrl: v.DetailType(),
				},
				RetryDelay: v.RetryDelay.String(),
			})

		case syserr.ResourceInfo:
			details = append(details, ResourceInfoResponse{
				DetailResponse: DetailResponse{
					TypeUrl: v.DetailType(),
				},
				ResourceType: v.ResourceType,
				ResourceName: v.ResourceName,
				Owner:        v.Owner,
				Description:  v.Description,
			})

		case syserr.RequestInfo:
			// intentionally omit ServingData from public response
			details = append(details, RequestInfoResponse{
				DetailResponse: DetailResponse{
					TypeUrl: v.DetailType(),
				},
				RequestID: v.RequestID,
			})

		case syserr.Help:
			details = append(details, HelpResponse{
				DetailResponse: DetailResponse{
					TypeUrl: v.DetailType(),
				},
				Links: lo.Map(v.Links, func(item syserr.HelpLink, _ int) HelpLinkResponse {
					return HelpLinkResponse{
						Description: item.Description,
						URL:         item.URL,
					}
				}),
			})

			// no syserr.DebugInfo here on purpose
		}
	}

	return details
}

// ParseSystemError parses a system error ([syserr.Error]) from an HTTP response.
func ParseSystemError(res *http.Response) (*syserr.Error, error) {
	if contentType := res.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		return nil, fmt.Errorf("bedrock.httpx: unsupported content type: %s", contentType)
	} else if res.Body == nil {
		return nil, fmt.Errorf("bedrock.httpx: empty response body")
	}

	var envelope struct {
		Error struct {
			Code       int               `json:"code"`
			Message    string            `json:"message"`
			StatusCode string            `json:"status"`
			Details    []json.RawMessage `json:"details"`
		} `json:"error"`
	}

	if err := sonic.ConfigDefault.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("bedrock.httpx: failed to decode system error response: %w", err)
	}

	out := &syserr.Error{
		Code:    syserr.Code(envelope.Error.StatusCode),
		Message: envelope.Error.Message,
	}

	for _, raw := range envelope.Error.Details {
		var meta DetailResponse
		if err := json.Unmarshal(raw, &meta); err != nil {
			continue
		}

		switch meta.TypeUrl {
		case syserr.ErrorInfoType:
			var in ErrorInfoResponse
			if err := json.Unmarshal(raw, &in); err != nil {
				continue
			}

			out.Details = append(out.Details, syserr.ErrorInfo{
				Reason:   in.Reason,
				Domain:   in.Domain,
				Metadata: in.Metadata,
			})

		case syserr.LocalizedMessageType:
			var in LocalizedMessageResponse
			if err := json.Unmarshal(raw, &in); err != nil {
				continue
			}

			out.Details = append(out.Details, syserr.LocalizedMessage{
				Locale:  language.Make(in.Locale),
				Message: in.Message,
			})

		case syserr.BadRequestType:
			var in BadRequestResponse
			if err := json.Unmarshal(raw, &in); err != nil {
				continue
			}

			violations := make([]syserr.FieldViolation, 0, len(in.Violations))
			for _, item := range in.Violations {
				v := syserr.FieldViolation{
					Field:       item.Field,
					Description: item.Description,
					Reason:      item.Reason,
				}

				if item.LocalizedMessage.Message != "" || item.LocalizedMessage.Locale != "" {
					v.LocalizedMessage = syserr.LocalizedMessage{
						Locale:  language.Make(item.LocalizedMessage.Locale),
						Message: item.LocalizedMessage.Message,
					}
				}

				violations = append(violations, v)
			}

			out.Details = append(out.Details, syserr.BadRequest{
				Violations: violations,
			})

		case syserr.PreconditionFailureType:
			var in PreconditionFailureResponse
			if err := json.Unmarshal(raw, &in); err != nil {
				continue
			}

			violations := make([]syserr.PreconditionViolation, 0, len(in.Violations))
			for _, item := range in.Violations {
				violations = append(violations, syserr.PreconditionViolation{
					Type:        item.Type,
					Subject:     item.Subject,
					Description: item.Description,
				})
			}

			out.Details = append(out.Details, syserr.PreconditionFailure{
				Violations: violations,
			})

		case syserr.QuotaFailureType:
			var in QuotaFailureResponse
			if err := json.Unmarshal(raw, &in); err != nil {
				continue
			}

			violations := make([]syserr.QuotaViolation, 0, len(in.Violations))
			for _, item := range in.Violations {
				violations = append(violations, syserr.QuotaViolation{
					Subject:     item.Subject,
					Description: item.Description,
				})
			}

			out.Details = append(out.Details, syserr.QuotaFailure{
				Violations: violations,
			})

		case syserr.RetryInfoType:
			var in RetryInfoResponse
			if err := json.Unmarshal(raw, &in); err != nil {
				continue
			}

			var retryDelay time.Duration
			if in.RetryDelay != "" {
				d, err := time.ParseDuration(in.RetryDelay)
				if err != nil {
					continue
				}
				retryDelay = d
			}

			out.Details = append(out.Details, syserr.RetryInfo{
				RetryDelay: retryDelay,
			})

		case syserr.ResourceInfoType:
			var in ResourceInfoResponse
			if err := json.Unmarshal(raw, &in); err != nil {
				continue
			}

			out.Details = append(out.Details, syserr.ResourceInfo{
				ResourceType: in.ResourceType,
				ResourceName: in.ResourceName,
				Owner:        in.Owner,
				Description:  in.Description,
			})

		case syserr.RequestInfoType:
			var in RequestInfoResponse
			if err := json.Unmarshal(raw, &in); err != nil {
				continue
			}

			out.Details = append(out.Details, syserr.RequestInfo{
				RequestID: in.RequestID,
			})

		case syserr.HelpType:
			var in HelpResponse
			if err := json.Unmarshal(raw, &in); err != nil {
				continue
			}

			links := make([]syserr.HelpLink, 0, len(in.Links))
			for _, item := range in.Links {
				links = append(links, syserr.HelpLink{
					Description: item.Description,
					URL:         item.URL,
				})
			}

			out.Details = append(out.Details, syserr.Help{
				Links: links,
			})

		case syserr.DebugInfoType:
			// Intentionally ignored in public HTTP parsing.
			// If you ever need this internally, make a separate ParseInternalSystemError.
			continue
		}
	}

	return out, nil
}
