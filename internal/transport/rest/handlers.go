package rest

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"

	"github.com/felipemalacarne/mesa/internal/application/commands"
	"github.com/felipemalacarne/mesa/internal/application/dtos"
	"github.com/felipemalacarne/mesa/internal/application/queries"
	"github.com/felipemalacarne/mesa/internal/transport/rest/contract"
	"github.com/felipemalacarne/mesa/web"
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

func (s *Server) ListConnections(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) FindConnection(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId) {
	id := uuid.UUID(connectionID)

	conn, err := s.app.Queries.FindConnection.Handle(r.Context(), queries.FindConnection{ConnectionID: id})
	if err != nil {
		log.Printf("ERROR: getConnection find connection %q: %v", connectionID, err)
		http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
		return
	}

	s.respondJSON(w, http.StatusOK, dtos.NewConnectionDTO(conn))
}

func (s *Server) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var body contract.CreateConnectionRequest

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := commands.CreateConnection{
		Name:     body.Name,
		Driver:   string(body.Driver),
		Host:     body.Host,
		Port:     body.Port,
		Username: body.Username,
		Password: body.Password,
	}

	conn, err := s.app.Commands.CreateConnection.Handle(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, http.StatusCreated, conn)
}

func (s *Server) ListDatabases(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId) {
	id := uuid.UUID(connectionID)

	databases, err := s.app.Queries.ListDatabases.Handle(r.Context(), queries.ListDatabases{ConnectionID: id})
	if err != nil {
		if errors.Is(err, queries.ErrConnectionNotFound) {
			log.Printf("ERROR: listDatabases connection not found %q", connectionID)
			http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
			return
		}

		log.Printf("WARN: listDatabases get databases %q: %v", connectionID, err)
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, databases)
}

func (s *Server) ListTables(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId, databaseName contract.DatabaseName) {
	dbName, err := url.PathUnescape(string(databaseName))
	if err != nil || dbName == "" {
		log.Printf("ERROR: listTables invalid database name %q: %v", databaseName, err)
		http.Error(w, "invalid database name", http.StatusBadRequest)
		return
	}

	id := uuid.UUID(connectionID)

	tables, err := s.app.Queries.ListTables.Handle(
		r.Context(),
		queries.ListTables{
			ConnectionID: id,
			DatabaseName: dbName,
		},
	)
	if err != nil {
		if errors.Is(err, queries.ErrConnectionNotFound) {
			log.Printf("ERROR: listTables connection not found %q", connectionID)
			http.Error(w, ErrConnectionNotFound, http.StatusNotFound)
			return
		}
		log.Printf("WARN: listTables get tables %q/%q: %v", connectionID, dbName, err)
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, tables)
}

func (s *Server) GetConnectionOverview(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId) {
	id := uuid.UUID(connectionID)

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

func (s *Server) ListSessions(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId) {
	id := uuid.UUID(connectionID)

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

func (s *Server) ListUsers(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId) {
	id := uuid.UUID(connectionID)

	users, err := s.app.Queries.ListUsers.Handle(r.Context(), queries.ListUsers{ConnectionID: id})
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

func (s *Server) CreateDatabase(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId) {
	id := uuid.UUID(connectionID)

	var body contract.CreateDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := commands.CreateDatabaseCmd{
		ConnectionID: id,
		Name:         body.Name,
		Owner:        body.Owner,
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

func (s *Server) CreateTable(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId, databaseName contract.DatabaseName) {
	id := uuid.UUID(connectionID)

	var body contract.CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := commands.CreateTableCmd{
		ConnectionID: id,
		DatabaseName: databaseName,
		Name:         body.Name,
		Columns:      mapTableColumns(body.Columns),
		Indexes:      mapTableIndexes(body.Indexes),
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

func (s *Server) CreateUser(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId) {
	id := uuid.UUID(connectionID)

	var body contract.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var isSuperUser bool
	if body.IsSuperuser != nil {
		isSuperUser = *body.IsSuperuser
	}

	cmd := commands.CreateUserCmd{
		ConnectionID: id,
		Username:     body.Username,
		Password:     body.Password,
		IsSuperUser:  isSuperUser,
		CanLogin:     body.CanLogin,
		ConnLimit:    body.ConnLimit,
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

func (s *Server) KillSession(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId, pid int) {
	id := uuid.UUID(connectionID)

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

func (s *Server) PingConnection(w http.ResponseWriter, r *http.Request, connectionID contract.ConnectionId) {
	id := uuid.UUID(connectionID)

	err := s.app.Queries.PingConnection.Handle(r.Context(), queries.PingConnection{ConnectionID: id})
	if err != nil {
		log.Printf("WARN: pingConnection %s: %v", id, err)
		s.respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
