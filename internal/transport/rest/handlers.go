package rest

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/felipemalacarne/mesa/internal/application/commands"
	"github.com/felipemalacarne/mesa/internal/application/queries"
	"github.com/felipemalacarne/mesa/web"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidUUID         = "invalid UUID format"
	ErrConnectionNotFound  = "connection not found"
	ErrInternalServerError = "internal server error"
)

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte(`{"status": "ok"}`)); err != nil {
		log.Printf("healthCheck write response: %v", err)
	}
}

func (s *Server) webHandler(w http.ResponseWriter, r *http.Request) {
	publicFS := web.GetPublicFS()
	fileServer := http.FileServer(http.FS(publicFS))
	file, err := publicFS.Open(r.URL.Path[1:]) // Remove a "/" inicial
	if err != nil {
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
		return
	}
	if err := file.Close(); err != nil {
		log.Printf("webHandler close file %q: %v", r.URL.Path, err)
	}
	fileServer.ServeHTTP(w, r)
}

func (s *Server) listConnections(w http.ResponseWriter, r *http.Request) {
	var query queries.ListConnections
	// if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

	conns, err := s.app.Queries.ListConnections.Handle(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, http.StatusOK, conns)
}

func (s *Server) findConnection(w http.ResponseWriter, r *http.Request) {
	connectionID := chi.URLParam(r, "connectionID")

	id, err := uuid.Parse(connectionID)
	if err != nil {
		log.Printf("ERROR: getConnection parse uuid %q: %v", connectionID, err)
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	conn, err := s.app.Queries.FindConnection.Handle(r.Context(), queries.FindConnection{ConnectionID: id})
	if err != nil {
		log.Printf("ERROR: getConnection find connection %q: %v", connectionID, err)
		http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
		return
	}

	s.respondJSON(w, http.StatusOK, conn)
}

func (s *Server) createConnection(w http.ResponseWriter, r *http.Request) {
	var cmd commands.CreateConnection

	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := s.app.Commands.CreateConnection.Handle(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, http.StatusCreated, conn)
}

// func (s *Server) listDatabases(w http.ResponseWriter, r *http.Request) {
// 	connectionID := chi.URLParam(r, "connectionID")
//
// }
