package syserr

import "errors"

// Is checks if the given error is of type code.
func Is(err error, code Code) bool {
	if err == nil {
		return false
	}
	se, ok := errors.AsType[*Error](err)
	if !ok {
		return false
	}
	return se.Code == code
}

// IsDetailType checks if the given error has a [Detail] of the specified type T.
func IsDetailType[T Detail](err error) bool {
	if err == nil {
		return false
	}

	se, ok := errors.AsType[*Error](err)
	if !ok {
		return false
	}

	var target T
	for _, detail := range se.Details {
		if detail.DetailType() == target.DetailType() {
			return true
		}
	}
	return false
}

// IsPublicDetail checks if the given detail is public.
func IsPublicDetail(d Detail) bool {
	if d == nil {
		return false
	}
	switch d.DetailType() {
	case ErrorInfoType,
		BadRequestType,
		RequestInfoType,
		HelpType,
		LocalizedMessageType,
		PreconditionFailureType,
		QuotaFailureType,
		RetryInfoType,
		ResourceInfoType:
		return true
	default:
		return false
	}
}
