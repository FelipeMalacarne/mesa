package rest

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"github.com/felipemalacarne/mesa/internal/application/commands"
	"github.com/felipemalacarne/mesa/internal/application/dtos"
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

func (s *Server) listDatabases(w http.ResponseWriter, r *http.Request) {
	connectionID := chi.URLParam(r, "connectionID")

	id, err := uuid.Parse(connectionID)
	if err != nil {
		log.Printf("ERROR: listDatabases parse uuid %q: %v", connectionID, err)
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	conn, err := s.app.Queries.FindConnection.Handle(r.Context(), queries.FindConnection{ConnectionID: id})
	if err != nil {
		log.Printf("ERROR: listDatabases find connection %q: %v", connectionID, err)
		http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
		return
	}

	databases, err := s.app.Queries.ListDatabases.Handle(r.Context(), queries.ListDatabases{}, *conn)
	if err != nil {
		log.Printf("WARN: listDatabases get databases %q: %v", connectionID, err)
		s.respondJSON(w, http.StatusOK, dtos.NewListDatabasesErrorResponse(err))
		return
	}

	databaseDTOs := make([]*dtos.DatabaseDTO, len(databases))
	for i, database := range databases {
		databaseDTOs[i] = dtos.NewDatabaseDTO(database)
	}

	s.respondJSON(w, http.StatusOK, dtos.NewListDatabasesResponse(databaseDTOs))
}

func (s *Server) listTables(w http.ResponseWriter, r *http.Request) {
	connectionID := chi.URLParam(r, "connectionID")
	databaseNameParam := chi.URLParam(r, "databaseName")

	databaseName, err := url.PathUnescape(databaseNameParam)
	if err != nil || databaseName == "" {
		log.Printf("ERROR: listTables invalid database name %q: %v", databaseNameParam, err)
		http.Error(w, "invalid database name", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(connectionID)
	if err != nil {
		log.Printf("ERROR: listTables parse uuid %q: %v", connectionID, err)
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	conn, err := s.app.Queries.FindConnection.Handle(r.Context(), queries.FindConnection{ConnectionID: id})
	if err != nil {
		log.Printf("ERROR: listTables find connection %q: %v", connectionID, err)
		http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
		return
	}

	tables, err := s.app.Queries.ListTables.Handle(
		r.Context(),
		queries.ListTables{DatabaseName: databaseName},
		*conn,
	)
	if err != nil {
		log.Printf("WARN: listTables get tables %q/%q: %v", connectionID, databaseName, err)
		s.respondJSON(w, http.StatusOK, dtos.NewListTablesErrorResponse(err))
		return
	}

	tableDTOs := make([]*dtos.TableDTO, len(tables))
	for i, table := range tables {
		tableDTOs[i] = dtos.NewTableDTO(table)
	}

	s.respondJSON(w, http.StatusOK, dtos.NewListTablesResponse(tableDTOs))
}
