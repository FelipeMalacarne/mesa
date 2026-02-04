package main

import (
	"context"
	"fmt"
	"log"

	"github.com/felipemalacarne/mesa/internal/application"
	"github.com/felipemalacarne/mesa/internal/config"
	"github.com/felipemalacarne/mesa/internal/infrastructure/crypto"
	"github.com/felipemalacarne/mesa/internal/infrastructure/postgres"
	"github.com/felipemalacarne/mesa/internal/transport/rest"
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

	repos := application.Repositories{
		Connection: postgres.NewConnectionRepository(db),
	}
	log.Println("Repositories initialized.")

	crypto, err := crypto.NewAESManager(cfg.AppKey)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to initialize crypto manager: %w", err))
	}
	log.Println("Crypto manager initialized.")

	app := application.NewApp(repos, crypto)
	log.Println("Application initialized.")

	srv := rest.NewServer(*app)

	err = srv.Start(cfg.Port)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to start server: %w", err))
	}
}
