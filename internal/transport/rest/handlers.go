package rest

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"

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

	s.respondJSON(w, http.StatusOK, dtos.NewConnectionDTO(conn))
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

	s.respondJSON(w, http.StatusOK, dtos.NewListDatabasesResponse(databases))
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

	s.respondJSON(w, http.StatusOK, dtos.NewListTablesResponse(tables))
}

func (s *Server) getConnectionOverview(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "connectionID"))
	if err != nil {
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	dto, err := s.app.Queries.GetOverview.Handle(r.Context(), id)
	if err != nil {
		if errors.Is(err, queries.ErrConnectionNotFound) {
			http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
			return
		}
		http.Error(w, ErrInternalServerError, http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, http.StatusOK, dto)
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "connectionID"))
	if err != nil {
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	sessions, err := s.app.Queries.ListSessions.Handle(r.Context(), id)
	if err != nil {
		if errors.Is(err, queries.ErrConnectionNotFound) {
			http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
			return
		}
		log.Printf("WARN: listSessions connection %s: %v", id, err)
		http.Error(w, "failed to reach remote database", http.StatusBadGateway)
		return
	}

	s.respondJSON(w, http.StatusOK, sessions)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "connectionID"))
	if err != nil {
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	users, err := s.app.Queries.ListUsers.Handle(r.Context(), id)
	if err != nil {
		if errors.Is(err, queries.ErrConnectionNotFound) {
			http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
			return
		}

		log.Printf("WARN: listUsers connection %s: %v", id, err)
		http.Error(w, ErrInternalServerError, http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, http.StatusOK, users)
}

func (s *Server) createDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "connectionID"))
	if err != nil {
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	var payload struct {
		Name  string `json:"name"`
		Owner string `json:"owner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := commands.CreateDatabaseCmd{
		ConnectionID: id,
		Name:         payload.Name,
		Owner:        payload.Owner,
	}

	if err := s.app.Commands.CreateDatabase.Handle(r.Context(), cmd); err != nil {
		if errors.Is(err, commands.ErrConnectionNotFound) {
			http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) createTable(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "connectionID"))
	if err != nil {
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	databaseName := chi.URLParam(r, "databaseName")
	if databaseName == "" {
		http.Error(w, "database name is required", http.StatusBadRequest)
		return
	}

	var payload struct {
		Name    string                 `json:"name"`
		Columns []commands.TableColumn `json:"columns"`
		Indexes []commands.TableIndex  `json:"indexes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := commands.CreateTableCmd{
		ConnectionID: id,
		DatabaseName: databaseName,
		Name:         payload.Name,
		Columns:      payload.Columns,
		Indexes:      payload.Indexes,
	}

	if err := s.app.Commands.CreateTable.Handle(r.Context(), cmd); err != nil {
		if errors.Is(err, commands.ErrConnectionNotFound) {
			http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
			return
		}
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "connectionID"))
	if err != nil {
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	var payload struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		IsSuperUser bool   `json:"is_superuser"`
		CanLogin    *bool  `json:"can_login"`
		ConnLimit   *int   `json:"conn_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := commands.CreateUserCmd{
		ConnectionID: id,
		Username:     payload.Username,
		Password:     payload.Password,
		IsSuperUser:  payload.IsSuperUser,
		CanLogin:     payload.CanLogin,
		ConnLimit:    payload.ConnLimit,
	}

	if err := s.app.Commands.CreateUser.Handle(r.Context(), cmd); err != nil {
		if errors.Is(err, commands.ErrConnectionNotFound) {
			http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) killSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "connectionID"))
	if err != nil {
		http.Error(w, ErrInvalidUUID, http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(chi.URLParam(r, "pid"))
	if err != nil {
		http.Error(w, "invalid pid", http.StatusBadRequest)
		return
	}

	cmd := commands.KillSessionCmd{
		ConnectionID: id,
		PID:          pid,
	}

	if err := s.app.Commands.KillSession.Handle(r.Context(), cmd); err != nil {
		if errors.Is(err, commands.ErrConnectionNotFound) {
			http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
