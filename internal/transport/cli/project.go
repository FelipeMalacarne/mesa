package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// initCmd creates the optional .mesa/ folder (ADR 0005).
func initCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [--from name[@env]]",
		Short: "Keep models, migrations and queries for this folder in .mesa/ (optional)",
		Args:  cobra.NoArgs,
		RunE: withApp(d, func(ctx context.Context, a *app.App, cmd *cobra.Command, _ []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("name")
			fromRef, _ := cmd.Flags().GetString("from")
			var from *connection.Connection
			if fromRef != "" {
				it, err := resolve(ctx, a, []string{fromRef})
				if err != nil {
					return err
				}
				from = &it.Connection
				if name == "" {
					name = it.Name
				}
			}
			p, err := app.InitProject(root, name, from)
			if err != nil {
				return err
			}
			fmt.Fprintf(d.Out, "%s created %s for project %s\n", symbolOK, p.Rel(p.Dir), p.Name)
			if from != nil {
				m, err := a.Designer.Model(ctx, from.ID, "")
				if err != nil {
					fmt.Fprintf(d.Err, "%s could not pull the model: %s\n", symbolWarn, errs.Message(err))
				} else if err := os.WriteFile(p.ModelPath(from.Database), []byte(m.DBML), 0o644); err == nil {
					c := m.Schema.Counts()
					fmt.Fprintf(d.Out, "%s %d tables, %d relations written to %s\n", symbolOK, c.Tables, c.ForeignKeys, p.Rel(p.ModelPath(from.Database)))
				}
				for _, c := range p.Connections() {
					if ref, _ := p.Secret(c.ID); ref != "" {
						fmt.Fprintf(d.Out, "\n  The project's password is the reference %s: set it (empty for trust auth)\n  before using the connection. The file never holds the secret.\n", ref)
					}
				}
			} else {
				fmt.Fprintln(d.Out, "\n  Add connections under [connections.<environment>] in .mesa/mesa.toml.")
			}
			fmt.Fprintln(d.Out, "  Then: mesa diff <env> · mesa apply <env> (migrations land in .mesa/migrations)")
			return nil
		}),
	}
	cmd.Flags().String("name", "", "project name (default: the connection's name or the folder's)")
	cmd.Flags().String("from", "", "start with this connection's settings and pull its model")
	return cmd
}
