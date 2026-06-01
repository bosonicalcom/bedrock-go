package syserr

// Global reason codes for system errors.
// These are stable UPPER_SNAKE_CASE identifiers intended for use in [ErrorInfo.Reason].
const (
	// ReasonUnknown is used when the cause of the error cannot be determined.
	ReasonUnknown = "UNKNOWN"

	// ReasonValidationFailed indicates that one or more input fields failed validation.
	ReasonValidationFailed = "VALIDATION_FAILED"

	// ReasonInvalidInput indicates the provided input is not acceptable in its current form.
	ReasonInvalidInput = "INVALID_INPUT"

	// ReasonRequiredValue indicates a required field or parameter was absent.
	ReasonRequiredValue = "REQUIRED_VALUE"

	// ReasonInvalidFormat indicates the value does not conform to the expected format (e.g. email, UUID).
	ReasonInvalidFormat = "INVALID_FORMAT"

	// ReasonNotOneOfValue indicates the value is not one of the accepted enumerated values.
	ReasonNotOneOfValue = "NOT_ONE_OF_VALUES"

	// ReasonInvalidLength indicates the length of a string, slice, or map is outside the allowed range.
	ReasonInvalidLength = "INVALID_LENGTH"

	// ReasonValueTooSmall indicates a numeric value is below the allowed minimum.
	ReasonValueTooSmall = "VALUE_TOO_SMALL"

	// ReasonValueTooLarge indicates a numeric value exceeds the allowed maximum.
	ReasonValueTooLarge = "VALUE_TOO_LARGE"

	// ReasonValueOutOfRange indicates a value falls outside an acceptable range.
	ReasonValueOutOfRange = "VALUE_OUT_OF_RANGE"

	// ReasonValueMismatch indicates a value does not match an expected counterpart (e.g. confirm password).
	ReasonValueMismatch = "VALUE_MISMATCH"

	// ReasonForbiddenValue indicates the value is explicitly disallowed.
	ReasonForbiddenValue = "FORBIDDEN_VALUE"

	// ReasonInvalidRelation indicates a cross-field constraint was violated.
	ReasonInvalidRelation = "INVALID_RELATION"

	// ReasonDuplicateValue indicates the value is already present and must be unique.
	ReasonDuplicateValue = "DUPLICATE_VALUE"
)
