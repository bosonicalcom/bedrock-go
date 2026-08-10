package grpcx

import (
	"github.com/samber/lo"
	"golang.org/x/text/language"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
	durationpb "google.golang.org/protobuf/types/known/durationpb"

	"github.com/bosonicalcom/bedrock-go/syserr"
)

func codeFromSystemError(code syserr.Code) codes.Code {
	switch code {
	case syserr.CodeCancelled:
		return codes.Canceled
	case syserr.CodeUnknown:
		return codes.Unknown
	case syserr.CodeInvalidArgument:
		return codes.InvalidArgument
	case syserr.CodeDeadlineExceeded:
		return codes.DeadlineExceeded
	case syserr.CodeNotFound:
		return codes.NotFound
	case syserr.CodeAlreadyExists:
		return codes.AlreadyExists
	case syserr.CodePermissionDenied:
		return codes.PermissionDenied
	case syserr.CodeUnauthenticated:
		return codes.Unauthenticated
	case syserr.CodeResourceExhausted:
		return codes.ResourceExhausted
	case syserr.CodeFailedPrecondition:
		return codes.FailedPrecondition
	case syserr.CodeAborted:
		return codes.Aborted
	case syserr.CodeOutOfRange:
		return codes.OutOfRange
	case syserr.CodeUnimplemented:
		return codes.Unimplemented
	case syserr.CodeUnavailable:
		return codes.Unavailable
	case syserr.CodeDataLoss:
		return codes.DataLoss
	default:
		return codes.Internal
	}
}

func syserrCodeFromGRPC(code codes.Code) syserr.Code {
	switch code {
	case codes.Canceled:
		return syserr.CodeCancelled
	case codes.Unknown:
		return syserr.CodeUnknown
	case codes.InvalidArgument:
		return syserr.CodeInvalidArgument
	case codes.DeadlineExceeded:
		return syserr.CodeDeadlineExceeded
	case codes.NotFound:
		return syserr.CodeNotFound
	case codes.AlreadyExists:
		return syserr.CodeAlreadyExists
	case codes.PermissionDenied:
		return syserr.CodePermissionDenied
	case codes.Unauthenticated:
		return syserr.CodeUnauthenticated
	case codes.ResourceExhausted:
		return syserr.CodeResourceExhausted
	case codes.FailedPrecondition:
		return syserr.CodeFailedPrecondition
	case codes.Aborted:
		return syserr.CodeAborted
	case codes.OutOfRange:
		return syserr.CodeOutOfRange
	case codes.Unimplemented:
		return syserr.CodeUnimplemented
	case codes.Unavailable:
		return syserr.CodeUnavailable
	case codes.DataLoss:
		return syserr.CodeDataLoss
	default:
		return syserr.CodeInternal
	}
}

// -- Locale conversion

// localeString renders a [language.Tag] for the wire. The zero Tag stringifies to "und",
// a valid-but-meaningless BCP 47 tag, so emit an empty string instead to keep an unset
// locale from round-tripping into a bogus one on the receiving side.
func localeString(tag language.Tag) string {
	if s := tag.String(); s != "und" {
		return s
	}
	return ""
}

// localeTag parses a BCP 47 tag received from the wire. Empty and unparseable values both
// yield [language.Und], matching the behavior of the httpx transport.
func localeTag(s string) language.Tag {
	if s == "" {
		return language.Und
	}
	return language.Make(s)
}

// protoLocalizedMessage converts a [syserr.LocalizedMessage] to its proto counterpart,
// returning nil for an unset message so it is omitted from the wire entirely.
func protoLocalizedMessage(msg syserr.LocalizedMessage) *errdetails.LocalizedMessage {
	if msg.Message == "" {
		return nil
	}
	return &errdetails.LocalizedMessage{
		Locale:  localeString(msg.Locale),
		Message: msg.Message,
	}
}

// systemLocalizedMessage converts a proto localized message to its [syserr] counterpart.
func systemLocalizedMessage(msg *errdetails.LocalizedMessage) syserr.LocalizedMessage {
	if msg == nil {
		return syserr.LocalizedMessage{}
	}
	return syserr.LocalizedMessage{
		Locale:  localeTag(msg.Locale),
		Message: msg.Message,
	}
}

// ParseSystemError parses a [syserr.Error] from an error returned by a gRPC client call.
//
//   - nil in, nil out.
//   - A gRPC status error is decoded into its code, message and details.
//   - A non-status error is still converted rather than discarded: context errors map to
//     CANCELLED and DEADLINE_EXCEEDED, everything else to UNKNOWN.
//
// The original error is always attached via [syserr.WithCauses], so errors.Is, errors.As
// and status.Code keep working through the returned error.
func ParseSystemError(err error) *syserr.Error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		// status.FromError already produced New(codes.Unknown, err.Error()); upgrade
		// context errors to the codes gRPC would have used for them.
		if ctxSt := status.FromContextError(err); ctxSt.Code() != codes.Unknown {
			st = ctxSt
		}
	}

	out := &syserr.Error{
		Code:    syserrCodeFromGRPC(st.Code()),
		Message: st.Message(),
		Causes:  []error{err},
	}

	for _, detail := range st.Details() {
		switch v := detail.(type) {
		case *errdetails.ErrorInfo:
			out.Details = append(out.Details, syserr.ErrorInfo{
				Reason:   v.Reason,
				Domain:   v.Domain,
				Metadata: v.Metadata,
			})

		case *errdetails.BadRequest:
			violations := make([]syserr.FieldViolation, 0, len(v.FieldViolations))
			for _, fv := range v.FieldViolations {
				violations = append(violations, syserr.FieldViolation{
					Field:            fv.Field,
					Description:      fv.Description,
					Reason:           fv.Reason,
					LocalizedMessage: systemLocalizedMessage(fv.LocalizedMessage),
				})
			}
			out.Details = append(out.Details, syserr.BadRequest{Violations: violations})

		case *errdetails.PreconditionFailure:
			violations := make([]syserr.PreconditionViolation, 0, len(v.Violations))
			for _, pv := range v.Violations {
				violations = append(violations, syserr.PreconditionViolation{
					Type:        pv.Type,
					Subject:     pv.Subject,
					Description: pv.Description,
				})
			}
			out.Details = append(out.Details, syserr.PreconditionFailure{Violations: violations})

		case *errdetails.QuotaFailure:
			violations := make([]syserr.QuotaViolation, 0, len(v.Violations))
			for _, qv := range v.Violations {
				violations = append(violations, syserr.QuotaViolation{
					Subject:     qv.Subject,
					Description: qv.Description,
				})
			}
			out.Details = append(out.Details, syserr.QuotaFailure{Violations: violations})

		case *errdetails.RetryInfo:
			out.Details = append(out.Details, syserr.RetryInfo{
				RetryDelay: v.RetryDelay.AsDuration(),
			})

		case *errdetails.ResourceInfo:
			out.Details = append(out.Details, syserr.ResourceInfo{
				ResourceType: v.ResourceType,
				ResourceName: v.ResourceName,
				Owner:        v.Owner,
				Description:  v.Description,
			})

		case *errdetails.RequestInfo:
			out.Details = append(out.Details, syserr.RequestInfo{
				RequestID:   v.RequestId,
				ServingData: v.ServingData,
			})

		case *errdetails.Help:
			links := make([]syserr.HelpLink, 0, len(v.Links))
			for _, l := range v.Links {
				links = append(links, syserr.HelpLink{
					Description: l.Description,
					URL:         l.Url,
				})
			}
			out.Details = append(out.Details, syserr.Help{Links: links})

		case *errdetails.LocalizedMessage:
			out.Details = append(out.Details, systemLocalizedMessage(v))

		case *errdetails.DebugInfo:
			out.Details = append(out.Details, syserr.DebugInfo{
				Detail:       v.Detail,
				StackEntries: v.StackEntries,
			})
		}
	}

	return out
}

// statusFromSystemError converts a [syserr.Error] to a gRPC [status.Status], including public details.
func statusFromSystemError(err *syserr.Error) *status.Status {
	st := status.New(codeFromSystemError(err.Code), err.Message)

	// Attach details one at a time: WithDetails is all-or-nothing per call, so a single
	// unmarshalable message must not take the rest of them down with it.
	for _, detail := range protoDetailsFromSystemError(err) {
		if enriched, e := st.WithDetails(detail); e == nil {
			st = enriched
		}
	}

	return st
}

func protoDetailsFromSystemError(err *syserr.Error) []protoadapt.MessageV1 {
	var details []protoadapt.MessageV1

	for _, detail := range err.Details {
		if !syserr.IsPublicDetail(detail) {
			continue
		}

		switch v := detail.(type) {
		case syserr.ErrorInfo:
			if v.Reason == "" {
				continue
			}
			details = append(details, &errdetails.ErrorInfo{
				Reason:   v.Reason,
				Domain:   v.Domain,
				Metadata: v.Metadata,
			})

		case syserr.LocalizedMessage:
			if v.Message == "" {
				continue
			}
			details = append(details, &errdetails.LocalizedMessage{
				Locale:  localeString(v.Locale),
				Message: v.Message,
			})

		case syserr.BadRequest:
			details = append(details, &errdetails.BadRequest{
				FieldViolations: lo.Map(v.Violations, func(item syserr.FieldViolation, _ int) *errdetails.BadRequest_FieldViolation {
					return &errdetails.BadRequest_FieldViolation{
						Field:            item.Field,
						Description:      item.Description,
						Reason:           item.Reason,
						LocalizedMessage: protoLocalizedMessage(item.LocalizedMessage),
					}
				}),
			})

		case syserr.PreconditionFailure:
			details = append(details, &errdetails.PreconditionFailure{
				Violations: lo.Map(v.Violations, func(item syserr.PreconditionViolation, _ int) *errdetails.PreconditionFailure_Violation {
					return &errdetails.PreconditionFailure_Violation{
						Type:        item.Type,
						Subject:     item.Subject,
						Description: item.Description,
					}
				}),
			})

		case syserr.QuotaFailure:
			details = append(details, &errdetails.QuotaFailure{
				Violations: lo.Map(v.Violations, func(item syserr.QuotaViolation, _ int) *errdetails.QuotaFailure_Violation {
					return &errdetails.QuotaFailure_Violation{
						Subject:     item.Subject,
						Description: item.Description,
					}
				}),
			})

		case syserr.RetryInfo:
			details = append(details, &errdetails.RetryInfo{
				RetryDelay: durationpb.New(v.RetryDelay),
			})

		case syserr.ResourceInfo:
			details = append(details, &errdetails.ResourceInfo{
				ResourceType: v.ResourceType,
				ResourceName: v.ResourceName,
				Owner:        v.Owner,
				Description:  v.Description,
			})

		case syserr.RequestInfo:
			details = append(details, &errdetails.RequestInfo{
				RequestId:   v.RequestID,
				ServingData: v.ServingData,
			})

		case syserr.Help:
			details = append(details, &errdetails.Help{
				Links: lo.Map(v.Links, func(item syserr.HelpLink, _ int) *errdetails.Help_Link {
					return &errdetails.Help_Link{
						Description: item.Description,
						Url:         item.URL,
					}
				}),
			})

			// syserr.DebugInfo intentionally omitted from public status
		}
	}

	return details
}
