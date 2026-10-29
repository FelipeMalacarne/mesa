// Package cli is the terminal surface of Mesa. Every command calls the app
// layer; writes go through the same plan → policy → apply path as the web app.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/app/connections"
	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/platform"
)

// Deps are what the commands need, built lazily so `mesa key` and `--help`
// never open the store.
type Deps struct {
	Open func(ctx context.Context) (*app.App, error)
	Out  io.Writer
	Err  io.Writer
	In   io.Reader
}

func (d Deps) withDefaults() Deps {
	if d.Out == nil {
		d.Out = os.Stdout
	}
	if d.Err == nil {
		d.Err = os.Stderr
	}
	if d.In == nil {
		d.In = os.Stdin
	}
	return d
}

// Root builds the command tree.
func Root(d Deps) *cobra.Command {
	d = d.withDefaults()
	root := &cobra.Command{
		Use:           "mesa",
		Short:         "Mesa: the SQL control plane, from the terminal",
		Version:       platform.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("json", false, "machine-readable output")
	root.AddCommand(initCmd(d), connectionsCmd(d), queryCmd(d), modelCmd(d), diffCmd(d), applyCmd(d), auditCmd(d), keyCmd(d))
	return root
}

// Run executes the tree and maps errors to exit codes.
func Run(ctx context.Context, d Deps, args []string) int {
	d = d.withDefaults()
	root := Root(d)
	root.SetArgs(args)
	// The CLI opens Mesa's store itself: whoever runs it is the local
	// principal, with no sign-in.
	if err := root.ExecuteContext(account.With(ctx, account.Local())); err != nil {
		fmt.Fprintf(d.Err, "%s %s\n", symbolErr, errs.Message(err))
		if errs.Is(err, errs.NeedsConfirmation) {
			fmt.Fprintf(d.Err, "  re-run with --confirm %s\n", errs.Hint(err))
		}
		switch errs.KindOf(err) {
		case errs.Forbidden, errs.NeedsConfirmation:
			return 3
		case errs.Invalid, errs.NotFound:
			return 2
		}
		return 1
	}
	return 0
}

const (
	symbolOK   = "✓"
	symbolWarn = "!"
	symbolErr  = "✗"
)

// resolve finds a connection by "name" or "name@environment". Inside a
// project, an environment name ("staging") picks the project's connection
// for it, and no argument at all picks the project's default.
func resolve(ctx context.Context, a *app.App, args []string) (*connections.Item, error) {
	ref := ""
	if len(args) > 0 {
		ref = args[0]
	}
	if ref == "" {
		if a.Project == nil || a.Project.Default == "" {
			return nil, errs.New(errs.Invalid, "name a connection (run `mesa connections`), or run this inside a folder with .mesa/")
		}
		ref = a.Project.Name + "@" + a.Project.Default
	}
	name, env, _ := strings.Cut(ref, "@")
	items, err := a.Connections.List(ctx)
	if err != nil {
		return nil, err
	}
	var matches []connections.Item
	for _, it := range items {
		if it.Name == name && (env == "" || string(it.Environment) == strings.ToLower(env)) {
			matches = append(matches, it)
		}
	}
	if len(matches) == 0 && env == "" && a.Project != nil {
		if c, err := a.Project.Connection(name); err == nil {
			for _, it := range items {
				if it.ID == c.ID {
					matches = append(matches, it)
				}
			}
		}
	}
	// Inside a project, its own connection wins over a same-named one of yours.
	if len(matches) > 1 {
		var fromProject []connections.Item
		for _, m := range matches {
			if m.DefinedIn != "" {
				fromProject = append(fromProject, m)
			}
		}
		if len(fromProject) == 1 {
			matches = fromProject
		}
	}
	switch len(matches) {
	case 0:
		return nil, errs.Newf(errs.NotFound, "no connection named %q (run `mesa connections`)", ref)
	case 1:
		return &matches[0], nil
	}
	envs := make([]string, 0, len(matches))
	for _, m := range matches {
		envs = append(envs, name+"@"+string(m.Environment))
	}
	return nil, errs.Newf(errs.Invalid, "%q matches several connections: %s", ref, strings.Join(envs, ", "))
}

func dbFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("db")
	return v
}

func jsonFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("json")
	return v
}
