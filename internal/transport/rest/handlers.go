package rest

import (
	"log"
	"net/http"

	"github.com/felipemalacarne/mesa/internal/application/queries"
	"github.com/felipemalacarne/mesa/web"
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
