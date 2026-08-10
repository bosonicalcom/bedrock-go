package syserr

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/text/language"
)

const (
	// GlobalDomain is a generic/global domain for errors that do not belong to a specific system or service.
	GlobalDomain = "bosonical.com"
)

// tiny interface the stdlib [errors] package uses but does not export.
type unwrapper interface {
	Unwrap() error
}

// An Error is a fault that occurred during the execution of a system operation.
// This type in specific lets the user create and manipulate structured error information, with fine-grained
// control over error codes, messages, and additional details that can be helpful for debugging and logging.
type Error struct {
	Code    Code
	Message string
	Details []Detail
	Causes  []error
}

var (
	_ error        = (*Error)(nil)
	_ fmt.Stringer = (*Error)(nil)
	_ unwrapper    = (*Error)(nil)
)

// New creates a new Error with the given code and message.
//
// It accepts variadic options to add additional details to the error.
// Example:
//
//	err := New(CodeNotFound, "Resource not found", WithDetails(DebugInfo{StackEntries: []string{"stack1"}, Detail: "more info"}))
func New(code Code, message string, opts ...NewOption) *Error {
	err := &Error{
		Code:    code,
		Message: message,
	}

	for _, opt := range opts {
		opt(err)
	}

	return err
}

// NewOption is a function that modifies an Error object before it is returned.
type NewOption func(*Error)

// -- Options

// WithDetails appends details to the error. This is useful for adding structured information.
func WithDetails(details ...Detail) NewOption {
	return func(err *Error) {
		if err.Details == nil {
			err.Details = make([]Detail, 0, len(details))
		}
		err.Details = append(err.Details, details...)
	}
}

// WithCauses appends one or many static errors to [Error]. This is useful for wrapping other errors.
func WithCauses(errs ...error) NewOption {
	return func(err *Error) {
		if err.Causes == nil {
			err.Causes = make([]error, 0, len(errs))
		}
		err.Causes = append(err.Causes, errs...)
	}
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) String() string {
	return e.Message
}

func (e Error) Unwrap() error {
	if len(e.Causes) > 0 {
		return errors.Join(e.Causes...)
	}
	return nil
}

// PublicDetails returns the subset of details that are safe to expose to external callers.
func (e Error) PublicDetails() []Detail {
	out := make([]Detail, 0, len(e.Details))
	for i := range e.Details {
		if ok := IsPublicDetail(e.Details[i]); !ok {
			continue // skip private
		}
		out = append(out, e.Details[i])
	}
	return out
}

// PrivateDetails returns the subset of details that must not be sent to external callers (e.g. [DebugInfo]).
func (e Error) PrivateDetails() []Detail {
	out := make([]Detail, 0, len(e.Details))
	for i := range e.Details {
		if ok := IsPublicDetail(e.Details[i]); ok {
			continue // skip non-private
		}
		out = append(out, e.Details[i])
	}
	return out
}

// - Detail

const (
	// ErrorInfoType identifies structured metadata about the error category,
	// classification domain, and additional key-value context.
	ErrorInfoType = "com.bosonical.error.ErrorInfo"

	// DebugInfoType identifies diagnostic information intended for development
	// and troubleshooting, such as stack entries or internal details.
	DebugInfoType = "com.bosonical.error.DebugInfo"

	// LocalizedMessageType identifies a user-facing message localized for a
	// specific language or locale.
	LocalizedMessageType = "com.bosonical.error.LocalizedMessage"

	// BadRequestType identifies validation failures tied to specific request
	// fields or request structure.
	BadRequestType = "com.bosonical.error.BadRequest"

	// PreconditionFailureType identifies failures caused by unmet system,
	// resource, or operation preconditions.
	PreconditionFailureType = "com.bosonical.error.PreconditionFailure"

	// QuotaFailureType identifies failures caused by exceeded quotas, rate
	// limits, or other consumption limits.
	QuotaFailureType = "com.bosonical.error.QuotaFailure"

	// RetryInfoType identifies retry guidance, such as the minimum delay before
	// attempting the operation again.
	RetryInfoType = "com.bosonical.error.RetryInfo"

	// ResourceInfoType identifies metadata about a resource involved in the
	// failure, such as its type, name, owner, or description.
	ResourceInfoType = "com.bosonical.error.ResourceInfo"

	// RequestInfoType identifies request-scoped metadata useful for tracing,
	// correlation, or diagnostics.
	RequestInfoType = "com.bosonical.error.RequestInfo"

	// HelpType identifies links or references to documentation, support, or
	// remediation resources related to the error.
	HelpType = "com.bosonical.error.Help"
)

// A Detail is a structured information that can be added to an error.
type Detail interface {
	// DetailType returns the type of the detail. This is used to identify the type of the detail.
	DetailType() string
}

// -- Error Info

// ErrorInfo describes the cause of the error with structured details.
type ErrorInfo struct {
	// Reason is the reason of the error. This is a constant value that identifies the proximate cause of the error.
	// Error reasons are unique within particular domains. This should be at most 63 characters and should contain only
	// uppercase letters, numbers, and underscores, which represents UPPER_SNAKE_CASE naming convention.
	Reason string
	// The logical grouping to which the Reason belongs. Examples: "bosonicalapis.com", "bosonicalapis.com/workspaces".
	// If the error is generated by some common infrastructure, the error domain must be a
	// globally unique value that identifies the infrastructure. For Bosonical API
	// infrastructure, the error domain is "bosonicalapis.com".
	Domain string
	// Additional structured details about this error.
	//
	// Keys must match a regular expression of `[a-z][a-zA-Z0-9-_]+` but should
	// ideally be lowerCamelCase. Also, they must be limited to 64 characters in
	// length. When identifying the current value of an exceeded limit, the units
	// should be contained in the key, not the value.  For example, rather than
	// `{"instanceLimit": "100/request"}`, should be returned as,
	// `{"instanceLimitPerRequest": "100"}`, if the client exceeds the number of
	// instances that can be created in a single (batch) request.
	Metadata map[string]string
}

var _ Detail = (*ErrorInfo)(nil)

// DetailType implements [Detail].
func (e ErrorInfo) DetailType() string {
	return ErrorInfoType
}

// -- Debug Info

// DebugInfo is an optional information that can be attached to an error. It provides additional context
// for debugging purposes.
//
// It is not suitable for sending to clients, it is only for internal use.
type DebugInfo struct {
	// Detail is a human-readable explanation of the error.
	Detail string
	// StackEntries is a list of stack trace entries that led to this error. This can be useful for debugging.
	StackEntries []string
	// Metadata is a map of key-value pairs that provide additional context for debugging.
	Metadata map[string]string
}

var _ Detail = (*DebugInfo)(nil)

// DetailType implements [Detail].
func (d DebugInfo) DetailType() string {
	return DebugInfoType
}

// -- Localized

// LocalizedMessage provides a localized message in a specific locale that is safe to return to the user.
type LocalizedMessage struct {
	// The locale of the message in BCP 47 language tag format. For example, "en-US", "es-MX", etc.
	Locale language.Tag
	// The localized message that is safe to return to the user.
	Message string
}

var _ Detail = (*LocalizedMessage)(nil)

// DetailType implements [Detail].
func (l LocalizedMessage) DetailType() string {
	return LocalizedMessageType
}

// -- Bad Request

// A BadRequest represents a request that could not be understood by the server.
//
// It is used to indicate that the request was malformed or violates the service's constraints.
type BadRequest struct {
	// Violations is a list of violations that caused the request to be rejected.
	Violations []FieldViolation
}

var _ Detail = (*BadRequest)(nil)

// DetailType implements [Detail].
func (b BadRequest) DetailType() string {
	return BadRequestType
}

// A FieldViolation represents a single violation of a constraint in the request.
type FieldViolation struct {
	// The name of the field that caused the violation. For example, "user.name".
	Field string
	// A human-readable description of the constraint violation.
	Description string
	// The reason of the field-level error. This is a constant value that
	// identifies the proximate cause of the field-level error. It should
	// uniquely identify the type of the FieldViolation within the scope of the
	// com.bosonical.error.ErrorInfo.domain. This should be at most 63
	// characters and match a regular expression of `[A-Z][A-Z0-9_]+[A-Z0-9]`,
	// which represents UPPER_SNAKE_CASE.
	Reason string
	// The localized message that is safe to return to the user.
	LocalizedMessage LocalizedMessage
}

// -- Precondition Failure

// PreconditionFailure describes why an operation could not proceed because one
// or more required conditions were not satisfied.
type PreconditionFailure struct {
	// Violations is the list of failed preconditions.
	Violations []PreconditionViolation
}

var _ Detail = (*PreconditionFailure)(nil)

// DetailType implements [Detail].
func (p PreconditionFailure) DetailType() string {
	return PreconditionFailureType
}

// PreconditionViolation describes a single failed precondition.
type PreconditionViolation struct {
	// Type is a stable identifier for the kind of failed precondition.
	Type string
	// Subject identifies the entity that failed the precondition.
	Subject string
	// Description is a human-readable explanation of the failure.
	Description string
}

// -- Quota Failure

// QuotaFailure describes why an operation was rejected due to quota or rate limits.
type QuotaFailure struct {
	// Violations is the list of quota violations that caused the request to fail.
	Violations []QuotaViolation
}

var _ Detail = (*QuotaFailure)(nil)

// DetailType implements [Detail].
func (q QuotaFailure) DetailType() string {
	return QuotaFailureType
}

// QuotaViolation describes a single quota or rate limit violation.
type QuotaViolation struct {
	// Subject identifies the entity that exceeded the quota.
	Subject string
	// Description is a human-readable explanation of the quota violation.
	Description string
}

// -- Retry Info

// RetryInfo provides guidance about when the operation may be retried.
type RetryInfo struct {
	// RetryDelay is the minimum amount of time the caller should wait before retrying.
	RetryDelay time.Duration
}

var _ Detail = (*RetryInfo)(nil)

// DetailType implements [Detail].
func (r RetryInfo) DetailType() string {
	return RetryInfoType
}

// -- Resource Info

// ResourceInfo describes the resource involved in the failure.
type ResourceInfo struct {
	// ResourceType identifies the kind of resource involved.
	ResourceType string
	// ResourceName identifies the specific resource involved.
	ResourceName string
	// Owner identifies the owner of the resource, when applicable.
	Owner string
	// Description is a human-readable explanation of the resource-related failure.
	Description string
}

var _ Detail = (*ResourceInfo)(nil)

// DetailType implements [Detail].
func (r ResourceInfo) DetailType() string {
	return ResourceInfoType
}

// -- Request Info

// RequestInfo carries metadata about the request that produced the error.
type RequestInfo struct {
	// RequestID is a unique identifier for the request.
	RequestID string
	// ServingData contains implementation-specific data useful for diagnostics.
	ServingData string
}

var _ Detail = (*RequestInfo)(nil)

// DetailType implements [Detail].
func (r RequestInfo) DetailType() string {
	return RequestInfoType
}

// -- Help

// Help provides links to documentation or troubleshooting resources.
type Help struct {
	// Links is the list of relevant help resources.
	Links []HelpLink
}

var _ Detail = (*Help)(nil)

// DetailType implements [Detail].
func (h Help) DetailType() string {
	return HelpType
}

// HelpLink points to a single help resource.
type HelpLink struct {
	// Description is a short label for the help resource.
	Description string
	// URL is the location of the help resource.
	URL string
}
