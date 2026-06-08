package httpx

import "net/http"

type HealthController struct {
}

var _ Controller = (*HealthController)(nil)

func (h HealthController) RegisterEndpoints(registrar *http.ServeMux) {
	registrar.HandleFunc("GET /healthz", h.health)
	registrar.HandleFunc("GET /readyz", h.readiness)
}

func (h HealthController) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h HealthController) readiness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
