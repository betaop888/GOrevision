package proxy

import (
	"errors"
	"log"
	"net/http"

	"gorevision/internal/lb"
)

type Handler struct {
	balancer *lb.Balancer
}

func NewHandler(balancer *lb.Balancer) *Handler {
	return &Handler{balancer: balancer}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend, err := h.balancer.Pick(r)
	if err != nil {
		if errors.Is(err, lb.ErrNoHealthyBackends) {
			http.Error(w, "no healthy backends available", http.StatusServiceUnavailable)
			return
		}

		log.Printf("pick backend: %v", err)
		http.Error(w, "backend selection error", http.StatusInternalServerError)
		return
	}

	backend.IncrementConnections()
	defer backend.DecrementConnections()

	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Forwarded-Proto", scheme(r))
	r.Host = backend.URL.Host

	backend.Proxy.ServeHTTP(w, r)
}

func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}

	return "http"
}
