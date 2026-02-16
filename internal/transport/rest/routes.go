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
	r.Route("/{connectionID}", func(r chi.Router) {
		r.Get("/", s.findConnection)
		r.Get("/ping", s.pingConnection)
		r.Get("/overview", s.getConnectionOverview)
		r.Get("/databases", s.listDatabases)
		r.Post("/databases", s.createDatabase)
		r.Get("/databases/{databaseName}/tables", s.listTables)
		r.Post("/databases/{databaseName}/tables", s.createTable)
		r.Get("/users", s.listUsers)
		r.Post("/users", s.createUser)
		r.Get("/sessions", s.listSessions)
		r.Delete("/sessions/{pid}", s.killSession)
	})
}
