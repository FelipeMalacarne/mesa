package rest

import (
	"github.com/go-chi/chi/v5/middleware"
)

func (s *Server) RegisterMiddlewares() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.RealIP)
}
