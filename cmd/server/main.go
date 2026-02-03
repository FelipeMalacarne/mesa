package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/felipemalacarne/mesa/internal/config"
	"github.com/felipemalacarne/mesa/internal/domain/infrastructure/postgres"
	"github.com/felipemalacarne/mesa/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {

	cfg := config.Load()
	ctx := context.Background()

	db, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to connect to database: %w", err))
	}
	defer db.Close()
	log.Println("Connected to the database successfully.")

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

	// Servir o Frontend Embutido
	publicFS := web.GetPublicFS()
	fileServer := http.FileServer(http.FS(publicFS))

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		file, err := publicFS.Open(r.URL.Path[1:]) // Remove a "/" inicial
		if err != nil {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		file.Close()
		fileServer.ServeHTTP(w, r)
	})

	http.ListenAndServe(":8080", r)
}
