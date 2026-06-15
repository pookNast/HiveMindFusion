package gateway

import "net/http"

// setupRoutes registers all OpenAI-compatible routes on mux.
func (g *Gateway) setupRoutes(mux *http.ServeMux, p *Proxy) {
	mux.HandleFunc("/v1/chat/completions", p.routeRequest)
	mux.HandleFunc("/v1/completions", p.routeRequest)
	mux.HandleFunc("/v1/models", p.handleModels)
	mux.HandleFunc("/health", g.handleHealth)
}
