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
	Gateways   connection.GatewayFactory
}

type Queries struct {
	FindConnection  *queries.FindConnectionHandler
	ListConnections *queries.ListConnectionsHandler
	ListDatabases   *queries.ListDatabasesHandler
	ListTables      *queries.ListTablesHandler
	GetOverview     *queries.GetOverviewHandler
	ListSessions    *queries.ListSessionsHandler
	ListUsers       *queries.ListUsersHandler
	PingConnection  *queries.PingConnectionHandler
}

type Commands struct {
	CreateConnection *commands.CreateConnectionHandler
	KillSession      *commands.KillSessionHandler
	CreateUser       *commands.CreateUserHandler
	CreateDatabase   *commands.CreateDatabaseHandler
	CreateTable      *commands.CreateTableHandler
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
			ListDatabases:   queries.NewListDatabasesHandler(repos.Connection, crypto, repos.Gateways),
			ListTables:      queries.NewListTablesHandler(repos.Connection, crypto, repos.Gateways),
			GetOverview:     queries.NewGetOverviewHandler(repos.Connection, crypto, repos.Gateways),
			ListSessions:    queries.NewListSessionsHandler(repos.Connection, crypto, repos.Gateways),
			ListUsers:       queries.NewListUsersHandler(repos.Connection, crypto, repos.Gateways),
			PingConnection:  queries.NewPingConnectionHandler(repos.Connection, crypto, repos.Gateways),
		},
		Commands: Commands{
			CreateConnection: commands.NewCreateConnectionHandler(repos.Connection, crypto),
			KillSession:      commands.NewKillSessionHandler(repos.Connection, crypto, repos.Gateways),
			CreateUser:       commands.NewCreateUserHandler(repos.Connection, crypto, repos.Gateways),
			CreateDatabase:   commands.NewCreateDatabaseHandler(repos.Connection, crypto, repos.Gateways),
			CreateTable:      commands.NewCreateTableHandler(repos.Connection, crypto, repos.Gateways),
		},
	}

	return app
}
