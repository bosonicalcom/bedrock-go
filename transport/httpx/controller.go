// Package httpx provides HTTP transport primitives, including the Controller interface for endpoint registration.
package httpx

import "net/http"

// A Controller is a system component responsible for registering HTTP endpoints
// and interconnecting transport-layer mechanics with internal system components.
type Controller interface {
	// RegisterEndpoints registers all HTTP endpoints with the given router.
	RegisterEndpoints(router *http.ServeMux)
}
