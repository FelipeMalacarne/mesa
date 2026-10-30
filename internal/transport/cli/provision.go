package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/app/provision"
)

func provisionCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision <name[@env]> <database> --namespace <app namespace>",
		Short: "Create an app's database and role, and declare them in the cluster's GitOps repository",
		Long: `Creates the role (with a generated password) and the database on the connection's
CloudNativePG cluster, and commits their declaration to the repository the cluster is
reconciled from: the CNPG Database, the managed role, and the Secret <database>-db in the
app's namespace with DATABASE_URL. The connection needs a [connections.<name>.gitops]
table in MESA_CONNECTIONS, and Mesa needs MESA_GITHUB_TOKEN.`,
		Args: cobra.ExactArgs(2),
		RunE: withApp(d, func(ctx context.Context, a *app.App, cmd *cobra.Command, args []string) error {
			it, err := resolve(ctx, a, args[:1])
			if err != nil {
				return err
			}
			namespace, _ := cmd.Flags().GetString("namespace")
			confirmation, _ := cmd.Flags().GetString("confirm")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if unlock, _ := cmd.Flags().GetBool("unlock"); unlock && !dryRun {
				if _, err := a.Connections.Unlock(ctx, it.ID, 5*time.Minute); err != nil {
					return err
				}
			}
			res, err := a.Provision.Provision(ctx, provision.Input{
				ConnectionID: it.ID, Name: args[1], Namespace: namespace,
				Confirmation: confirmation, DryRun: dryRun, Actor: actorName(), Surface: surface,
			})
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return printJSON(d.Out, res)
			}
			t := res.Target
			fmt.Fprintf(d.Out, "%s on %s/%s, declared in %s@%s:%s\n", args[1], t.Namespace, t.Cluster, t.Repo, t.Branch, t.Path)
			fmt.Fprintln(d.Out, "\n  "+strings.Join(res.Statements, "\n  "))
			fmt.Fprintln(d.Out, "\n  "+strings.Join(res.Files, "\n  "))
			fmt.Fprintf(d.Out, "\n  Secret %s in namespace %s\n", res.AppSecret, namespace)
			if dryRun {
				fmt.Fprintf(d.Out, "\n%s dry run: %s\n", symbolWarn, res.Decision.Outcome)
				return nil
			}
			if c := res.Commit; c != nil {
				if c.PullRequestURL != "" {
					fmt.Fprintf(d.Out, "\n%s pull request %s\n", symbolOK, c.PullRequestURL)
				} else {
					fmt.Fprintf(d.Out, "\n%s committed %s\n", symbolOK, c.URL)
				}
			}
			sym := symbolOK
			if !res.Applied && !t.PullRequest {
				sym = symbolWarn
			}
			fmt.Fprintf(d.Out, "%s %s\n", sym, res.Note)
			return nil
		}),
	}
	cmd.Flags().String("namespace", "", "the app's Kubernetes namespace (gets the Secret <database>-db)")
	cmd.Flags().String("confirm", "", "the database name, when the policy asks for it (production)")
	cmd.Flags().Bool("unlock", false, "unlock a read-only connection for five minutes first")
	cmd.Flags().Bool("dry-run", false, "show what would run and be committed; do nothing")
	_ = cmd.MarkFlagRequired("namespace")
	return cmd
}
