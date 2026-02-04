// Package api provides the Rest http API layer for the application.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/felipemalacarne/mesa/internal/application"
	"github.com/felipemalacarne/mesa/internal/application/queries"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	app    application.App
	router chi.Router
	server *http.Server
}

func NewServer(app application.App) *Server {
	return &Server{
		app:    app,
		router: chi.NewRouter(),
	}
}

func (s *Server) Start(addr string) error {
	s.RegisterRoutes()

	s.server = &http.Server{
		Handler:      s.router,
		Addr:         addr,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	log.Println("Shutting down API server...")
	return s.server.Shutdown(ctx)
}

// routes
func (s *Server) RegisterRoutes() {
	s.router.Route("/api", func(r chi.Router) {
		r.Get("/health", s.healthCheck)
		r.Route("/connections", connectionsHandler)
	})
	// s.router.Get("/*") // todo serve static files
}

func connectionsHandler(r chi.Router) {
}

// handlers
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status": "ok"}`))
}

func (s *Server) listConnections(w http.ResponseWriter, r *http.Request) {
	var cmd queries.ListConnections
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conns, err := s.app.Queries.ListConnections.Handle(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, http.StatusOK, conns)
}

func (s *Server) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// func (s *Server) respondError(w http.ResponseWriter, status int, message string) {
// 	s.respondJSON(w, status, map[string]string{"error": message})
// }
