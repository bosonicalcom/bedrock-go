// Package syserr defines the structured error type and error code taxonomy used across the system.
package syserr

// Code is a stable, machine-readable string that classifies the kind of error.
// It is modelled after gRPC status codes and is intended to be used as a
// transport-agnostic error classification that can be mapped to HTTP status
// codes, gRPC codes, or any other protocol as needed.
type Code string

const (
	// CodeCancelled indicates the operation was canceled, typically by the caller.
	CodeCancelled Code = "CANCELLED"

	// CodeUnknown indicates an unknown error or an error that cannot be classified more precisely.
	CodeUnknown Code = "UNKNOWN"

	// CodeInvalidArgument indicates the client provided an invalid argument, regardless of system state.
	CodeInvalidArgument Code = "INVALID_ARGUMENT"

	// CodeDeadlineExceeded indicates the operation did not complete before the deadline expired.
	CodeDeadlineExceeded Code = "DEADLINE_EXCEEDED"

	// CodeNotFound indicates the requested resource does not exist.
	CodeNotFound Code = "NOT_FOUND"

	// CodeAlreadyExists indicates an attempt to create a resource that already exists.
	CodeAlreadyExists Code = "ALREADY_EXISTS"

	// CodePermissionDenied indicates the caller lacks permission to perform the operation.
	CodePermissionDenied Code = "PERMISSION_DENIED"

	// CodeUnauthenticated indicates the request is missing valid authentication credentials.
	CodeUnauthenticated Code = "UNAUTHENTICATED"

	// CodeResourceExhausted indicates some resource has been exhausted, such as quota or capacity.
	CodeResourceExhausted Code = "RESOURCE_EXHAUSTED"

	// CodeFailedPrecondition indicates the operation was rejected because the system is not in the required state.
	CodeFailedPrecondition Code = "FAILED_PRECONDITION"

	// CodeAborted indicates the operation was aborted, typically due to concurrency or transaction conflicts.
	CodeAborted Code = "ABORTED"

	// CodeOutOfRange indicates the operation was attempted past a valid range.
	CodeOutOfRange Code = "OUT_OF_RANGE"

	// CodeUnimplemented indicates the operation is not implemented or not supported.
	CodeUnimplemented Code = "UNIMPLEMENTED"

	// CodeInternal indicates an internal error occurred and the system invariants may have been broken.
	CodeInternal Code = "INTERNAL"

	// CodeUnavailable indicates the service is currently unavailable and the operation may succeed if retried.
	CodeUnavailable Code = "UNAVAILABLE"

	// CodeDataLoss indicates unrecoverable data loss or corruption.
	CodeDataLoss Code = "DATA_LOSS"
)
