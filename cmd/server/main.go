package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	fmt.Println("Server is starting...")
	r := chi.NewRouter()

	// Middlewares padrão de "Cloud Grade"
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP) // Essencial rodando atrás de um Ingress no K8s

	// Agrupamento da API
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status": "ok"}`))
		})

		// r.Route("/connections", connectionsHandler)
		// r.Route("/users", usersHandler)
	})

	// Servir o Frontend (React SPA)
	// Em produção, usaríamos o web.DistFS que discutimos antes
	// workDir, _ := os.Getwd()
	// filesDir := http.Dir(workDir + "/web/dist")
	// FileServer(r, "/", filesDir)

	http.ListenAndServe(":8080", r)
}
