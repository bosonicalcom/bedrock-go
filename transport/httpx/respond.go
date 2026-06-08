package httpx

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	rdebug "runtime/debug"
	"strings"

	"github.com/bosonicalcom/bedrock-go/mimex"
	"github.com/bosonicalcom/bedrock-go/syserr"
	"github.com/bytedance/sonic"
)

// RespondText sends v as the response in plain text format.
func RespondText(w http.ResponseWriter, status int, v string) {
	w.Header().Set("Content-Type", mimex.TextPlain.String())
	w.WriteHeader(status)
	_, _ = w.Write([]byte(v))
}

// RespondJSON encodes v and sends the response as JSON.
func RespondJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := sonic.ConfigDefault.NewEncoder(&buf).Encode(v); err != nil {
		RespondErrorJSON(w, fmt.Errorf("failed to encode response: %w", err))
		return
	}

	w.Header().Set("Content-Type", mimex.ApplicationJSON.String())
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// RespondErrorJSON encodes err and sends the response as JSON with an error status.
func RespondErrorJSON(w http.ResponseWriter, err error, opts ...RespondErrJSONOption) {
	options := &respondErrorJSONOptions{}
	for _, opt := range opts {
		opt(options)
	}

	sysErr, ok := errors.AsType[*syserr.Error](err)
	if !ok {
		// do not use debuginfo to avoid exposing internal details to clients
		// build a generic internal error to match what the client expects.
		sysErr = syserr.New(syserr.CodeInternal, http.StatusText(http.StatusInternalServerError),
			syserr.WithDetails(newInternalDebugInfo(err)),
			syserr.WithCauses(err),
		)
	}

	ensureRequestIDDetail(sysErr, options.requestID)
	if setter, ok := w.(interface{ SetSystemError(*syserr.Error) }); ok {
		setter.SetSystemError(sysErr)
	}

	code := getCodeFromSystemError(sysErr.Code)
	RespondJSON(w, code, ErrorEnvelope{
		Error: ErrorResponse{
			Code:       code,
			Message:    sysErr.Message,
			StatusCode: string(sysErr.Code),
			Details:    newDetailFromSystemError(sysErr),
		},
	})
}

type respondErrorJSONOptions struct {
	requestID string
}

type RespondErrJSONOption func(options *respondErrorJSONOptions)

// WithRequestID sets a request id to an error response built by [RespondErrorJSON].
func WithRequestID(id string) RespondErrJSONOption {
	return func(options *respondErrorJSONOptions) {
		options.requestID = id
	}
}

func ensureRequestIDDetail(sysErr *syserr.Error, requestID string) {
	if sysErr == nil || requestID == "" {
		return
	}

	for _, detail := range sysErr.Details {
		if requestInfo, ok := detail.(syserr.RequestInfo); ok && requestInfo.RequestID != "" {
			return
		}
	}

	sysErr.Details = append(sysErr.Details, syserr.RequestInfo{RequestID: requestID})
}

func newInternalDebugInfo(err error) syserr.DebugInfo {
	stack := strings.Split(strings.TrimSpace(string(rdebug.Stack())), "\n")
	return syserr.DebugInfo{
		Detail:       err.Error(),
		StackEntries: stack,
	}
}
