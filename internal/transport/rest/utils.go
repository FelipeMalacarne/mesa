package rest

import (
	"encoding/json"
	"log"
	"net/http"
)

func (s *Server) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("respondJSON encode response: %v", err)
		}
	}
}
