package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felipemalacarne/mesa/internal/application"
	"github.com/felipemalacarne/mesa/internal/config"
	"github.com/felipemalacarne/mesa/internal/infrastructure/crypto"
	"github.com/felipemalacarne/mesa/internal/infrastructure/gateway"
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
		Gateways:   gateway.NewFactory(),
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

	go func() {
		if err := srv.Start(cfg.Port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
