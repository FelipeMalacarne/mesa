// Package application provides the application layer of the Mesa project,
// which includes the business logic and use cases.
// It defines the structure of the application, including repositories, queries, and commands.
// The App struct serves as the main entry point for accessing the application's functionality.
package application

import (
	"github.com/felipemalacarne/mesa/internal/application/commands"
	"github.com/felipemalacarne/mesa/internal/application/queries"
	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type Repositories struct {
	Connection connection.Repository
}

type Queries struct {
	ListConnections *queries.ListConnectionsHandler
}

type Commands struct {
	CreateConnection *commands.CreateConnectionHandler
}

type App struct {
	Queries  Queries
	Commands Commands
}

func NewApp(repos Repositories, crypto domain.Cryptographer) *App {
	app := &App{
		Queries: Queries{
			ListConnections: queries.NewListConnectionsHandler(repos.Connection),
		},
		Commands: Commands{
			CreateConnection: commands.NewCreateConnectionHandler(repos.Connection, crypto),
		},
	}

	return app
}
