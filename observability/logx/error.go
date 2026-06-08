package logx

import (
	"errors"
	"log/slog"

	"github.com/bosonicalcom/bedrock-go/syserr"
)

type errorWrapper interface {
	Unwrap() []error
}

// Error returns a [slog.Attr] for an error value.
//
// Sets "error" as [slog.Attr] key. In addition, detects several error types to properly form a slog.Attr.
func Error(err error) slog.Attr {
	sysErr, ok := errors.AsType[*syserr.Error](err)
	if ok {
		detailAttrs := make([]slog.Attr, 0, len(sysErr.Details))
		detailTypes := make([]string, 0, len(sysErr.Details))
		for _, d := range sysErr.Details {
			detailTypes = append(detailTypes, d.DetailType())
			detailAttrs = append(detailAttrs, slog.Any(d.DetailType(), d))
		}

		causes := make([]string, 0, len(sysErr.Causes))
		for _, cause := range sysErr.Causes {
			causes = append(causes, cause.Error())
		}
		return slog.Group("error",
			slog.String("code", string(sysErr.Code)),
			slog.String("message", sysErr.Message),
			slog.Any("detail_types", detailTypes),
			slog.GroupAttrs("details", detailAttrs...),
			slog.Any("causes", causes),
		)
	}

	errs, ok := err.(errorWrapper)
	if !ok {
		return slog.Attr{
			Key:   "error",
			Value: slog.StringValue(err.Error()),
		}
	}

	attrs := make([]string, 0, len(errs.Unwrap()))
	for _, errUnwrapped := range errs.Unwrap() {
		attrs = append(attrs, errUnwrapped.Error())
	}
	return slog.Attr{
		Key:   "error",
		Value: slog.GroupValue(slog.Any("errors", attrs)),
	}
}
