package httpx

// ErrorEnvelope is the top-level error response wrapper.
// This matches the HTTP/1.1+JSON representation from AIP-193.
type ErrorEnvelope struct {
	Error ErrorResponse `json:"error"`
}

// ErrorResponse contains the actual error information.
// Note: The Code field is the HTTP status code, NOT the bosonical.rpc.Code numeric value.
type ErrorResponse struct {
	// Code is the HTTP status code (e.g., 400, 404, 500).
	// Important: This differs from gRPC where code is the numeric bosonical.rpc.Code value.
	Code int `json:"code"`

	// Message is a developer-facing, human-readable debug message.
	// Should be in English. Must remain stable if ErrorInfo was not always present.
	Message string `json:"message"`

	// StatusCode is the canonical error code name from bosonical.rpc.Code enum.
	// Examples: "NOT_FOUND", "INVALID_ARGUMENT", "PERMISSION_DENIED"
	StatusCode string `json:"status"`

	// Details contains additional error information.
	// Must include at least one ErrorInfo. Each detail type may appear at most once.
	Details []any `json:"details,omitempty"`
}

type DetailResponse struct {
	TypeUrl string `json:"@type"`
}

type ErrorInfoResponse struct {
	DetailResponse
	Reason   string            `json:"reason,omitempty"`
	Domain   string            `json:"domain,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type LocalizedMessageResponse struct {
	DetailResponse
	Locale  string `json:"locale,omitempty"`
	Message string `json:"message,omitempty"`
}

type BadRequestResponse struct {
	DetailResponse
	Violations []FieldViolationResponse `json:"violations,omitempty"`
}

type FieldViolationResponse struct {
	Field            string                   `json:"field,omitempty"`
	Description      string                   `json:"description,omitempty"`
	Reason           string                   `json:"reason,omitempty"`
	LocalizedMessage LocalizedMessageResponse `json:"localized_message,omitempty"`
}

type RequestInfoResponse struct {
	DetailResponse
	RequestID string `json:"request_id,omitempty"`
}

type PreconditionFailureResponse struct {
	DetailResponse
	Violations []PreconditionViolationResponse `json:"violations,omitempty"`
}

type PreconditionViolationResponse struct {
	Type        string `json:"type,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Description string `json:"description,omitempty"`
}

type QuotaFailureResponse struct {
	DetailResponse
	Violations []QuotaViolationResponse `json:"violations,omitempty"`
}

type QuotaViolationResponse struct {
	Subject     string `json:"subject,omitempty"`
	Description string `json:"description,omitempty"`
}

type RetryInfoResponse struct {
	DetailResponse
	RetryDelay string `json:"retry_delay,omitempty"`
}

type ResourceInfoResponse struct {
	DetailResponse
	ResourceType string `json:"resource_type,omitempty"`
	ResourceName string `json:"resource_name,omitempty"`
	Owner        string `json:"owner,omitempty"`
	Description  string `json:"description,omitempty"`
}

type HelpResponse struct {
	DetailResponse
	Links []HelpLinkResponse `json:"links,omitempty"`
}

type HelpLinkResponse struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}
