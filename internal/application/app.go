package application

import (
	"github.com/felipemalacarne/mesa/internal/application/queries"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type Repositories struct {
	Connection connection.Repository
}

type Queries struct {
	ListConnections *queries.ListConnectionsHandler
}

type Commands struct{}

type App struct {
	Queries  Queries
	Commands Commands
}

func NewApp(repos Repositories) *App {
	listConnectionsHandler := queries.NewListConnectionsHandler(repos.Connection)

	app := &App{
		Queries: Queries{
			ListConnections: listConnectionsHandler,
		},
		Commands: Commands{},
	}

	return app
}
