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
	Inspectors connection.InspectorFactory
}

type Queries struct {
	FindConnection  *queries.FindConnectionHandler
	ListConnections *queries.ListConnectionsHandler
	ListDatabases   *queries.ListDatabasesHandler
	ListTables      *queries.ListTablesHandler
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
			FindConnection:  queries.NewFindConnectionHandler(repos.Connection),
			ListConnections: queries.NewListConnectionsHandler(repos.Connection),
			ListDatabases:   queries.NewListDatabasesHandler(crypto, repos.Inspectors),
			ListTables:      queries.NewListTablesHandler(crypto, repos.Inspectors),
		},
		Commands: Commands{
			CreateConnection: commands.NewCreateConnectionHandler(repos.Connection, crypto),
		},
	}

	return app
}
