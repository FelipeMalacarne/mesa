package rest

import "github.com/go-chi/chi/v5"

func (s *Server) RegisterRoutes() {
	s.router.Route("/api", func(r chi.Router) {
		r.Get("/health", s.healthCheck)

		r.Route("/connections", s.connectionRoutes)
	})

	s.router.Get("/*", s.webHandler)
}

func (s *Server) connectionRoutes(r chi.Router) {
	r.Get("/", s.listConnections)
	r.Post("/", s.createConnection)
	r.Get("/{connectionID}", s.findConnection)
	// r.Get("/{connectionID}/databases", s.listDatabases)
}
