package rest

import (
	"github.com/felipemalacarne/mesa/internal/transport/rest/contract"
	"github.com/go-chi/chi/v5"
)

func (s *Server) RegisterRoutes() {
	s.router.Route("/api", func(r chi.Router) {
		r.Get("/health", s.healthCheck)

		contract.HandlerFromMux(s, r)
	})

	s.router.Get("/*", s.webHandler)
}
