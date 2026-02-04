// Package rest provides the Rest http API layer for the application.
package rest

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/felipemalacarne/mesa/internal/application"
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
	s.RegisterMiddlewares()
	s.RegisterRoutes()

	s.server = &http.Server{
		Handler:      s.router,
		Addr:         fmt.Sprintf(":%s", addr),
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
