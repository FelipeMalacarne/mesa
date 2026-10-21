package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/app/changes"
	"github.com/felipemalacarne/mesa/internal/app/connections"
	"github.com/felipemalacarne/mesa/internal/app/query"
	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/policy"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/dbml"
	"github.com/felipemalacarne/mesa/internal/secret"
)

const surface = "cli"

func withApp(d Deps, fn func(ctx context.Context, a *app.App, cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		a, err := d.Open(cmd.Context())
		if err != nil {
			return err
		}
		defer func() { _ = a.Close() }()
		return fn(cmd.Context(), a, cmd, args)
	}
}

// --- connections ----------------------------------------------------------

func connectionsCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connections",
		Aliases: []string{"conn", "connection"},
		Short:   "List connections",
		RunE: withApp(d, func(ctx context.Context, a *app.App, cmd *cobra.Command, _ []string) error {
			items, err := a.Connections.List(ctx)
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return printJSON(d.Out, items)
			}
			if len(items) == 0 {
				fmt.Fprintln(d.Out, "  no connections yet · add one with `mesa connections add`")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				ro := ""
				if it.ReadOnly {
					ro = "read-only"
					if it.UnlockedUntil != nil {
						ro = "unlocked"
					}
				}
				rows = append(rows, []string{it.Name, envDot(d.Out, it.Environment), string(it.Engine), fmt.Sprintf("%s:%d/%s", it.Host, it.Port, it.Database), ro})
			}
			table(d.Out, []string{"name", "environment", "engine", "target", ""}, rows, nil)
			return nil
		}),
	}
	add := &cobra.Command{
		Use:   "add",
		Short: "Add a connection",
		RunE: withApp(d, func(ctx context.Context, a *app.App, cmd *cobra.Command, _ []string) error {
			f := cmd.Flags()
			in := connections.Input{}
			in.Name, _ = f.GetString("name")
			in.Engine, _ = f.GetString("engine")
			in.Environment, _ = f.GetString("env")
			in.Host, _ = f.GetString("host")
			in.Port, _ = f.GetInt("port")
			in.Database, _ = f.GetString("database")
			in.User, _ = f.GetString("user")
			in.TLS, _ = f.GetString("tls")
			in.Secret, _ = f.GetString("secret")
			if ro, _ := f.GetBool("read-only"); f.Changed("read-only") {
				in.ReadOnly = &ro
			}
			if in.Secret == "" {
				if v := os.Getenv("MESA_PASSWORD"); v != "" {
					in.Secret = v
				} else if isTTY(d.Out) {
					fmt.Fprint(d.Out, "password (or env:NAME): ")
					line, _ := bufio.NewReader(d.In).ReadString('\n')
					in.Secret = strings.TrimSpace(line)
				}
			}
			it, err := a.Connections.Create(ctx, in)
			if err != nil {
				return err
			}
			fmt.Fprintf(d.Out, "%s added %s %s\n", symbolOK, it.Name, envDot(d.Out, it.Environment))
			return nil
		}),
	}
	add.Flags().String("name", "", "connection name")
	add.Flags().String("engine", "postgres", "postgres or mysql")
	add.Flags().String("env", "local", "local, development, staging or production")
	add.Flags().String("host", "localhost", "host")
	add.Flags().Int("port", 0, "port (engine default when 0)")
	add.Flags().String("database", "", "default database")
	add.Flags().String("user", "", "user")
	add.Flags().String("tls", "prefer", "disable, prefer, require, verify-ca, verify-full")
	add.Flags().String("secret", "", "password or env:NAME (else MESA_PASSWORD, else prompt)")
	add.Flags().Bool("read-only", false, "open read-only (always on for production)")
	_ = add.MarkFlagRequired("name")
	_ = add.MarkFlagRequired("user")

	rm := &cobra.Command{
		Use:   "rm <name[@env]>",
		Short: "Remove a connection",
		Args:  cobra.ExactArgs(1),
		RunE: withApp(d, func(ctx context.Context, a *app.App, _ *cobra.Command, args []string) error {
			it, err := resolve(ctx, a, args[0])
			if err != nil {
				return err
			}
			if err := a.Connections.Delete(ctx, it.ID); err != nil {
				return err
			}
			fmt.Fprintf(d.Out, "%s removed %s\n", symbolOK, it.Name)
			return nil
		}),
	}
	test := &cobra.Command{
		Use:   "test <name[@env]>",
		Short: "Reach the server and print its version and latency",
		Args:  cobra.ExactArgs(1),
		RunE: withApp(d, func(ctx context.Context, a *app.App, _ *cobra.Command, args []string) error {
			it, err := resolve(ctx, a, args[0])
			if err != nil {
				return err
			}
			res, err := a.Connections.Ping(ctx, it.ID)
			if err != nil {
				return err
			}
			fmt.Fprintf(d.Out, "%s %s %s · %s %s · %d ms\n", symbolOK, it.Name, envDot(d.Out, it.Environment), it.Engine, res.Overview.Version, res.Latency.Milliseconds())
			return nil
		}),
	}
	cmd.AddCommand(add, rm, test)
	return cmd
}

// --- query ----------------------------------------------------------------

func queryCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <name[@env]> <sql>",
		Short: "Run SQL (read-only connections run in a read-only transaction)",
		Args:  cobra.ExactArgs(2),
		RunE: withApp(d, func(ctx context.Context, a *app.App, cmd *cobra.Command, args []string) error {
			it, err := resolve(ctx, a, args[0])
			if err != nil {
				return err
			}
			max, _ := cmd.Flags().GetInt("limit")
			res, err := a.Query.Run(ctx, query.RunInput{ConnectionID: it.ID, Database: dbFlag(cmd), SQL: args[1], MaxRows: max})
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return printJSON(d.Out, res)
			}
			result(d.Out, res)
			return nil
		}),
	}
	cmd.Flags().String("db", "", "database (default: the connection's)")
	cmd.Flags().Int("limit", 500, "max rows")
	return cmd
}

// --- model ----------------------------------------------------------------

func modelCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "model", Short: "Schema models as DBML"}
	pull := &cobra.Command{
		Use:   "pull <name[@env]>",
		Short: "Print the live schema as DBML",
		Args:  cobra.ExactArgs(1),
		RunE: withApp(d, func(ctx context.Context, a *app.App, cmd *cobra.Command, args []string) error {
			it, err := resolve(ctx, a, args[0])
			if err != nil {
				return err
			}
			m, err := a.Designer.Model(ctx, it.ID, dbFlag(cmd))
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return printJSON(d.Out, m.Schema)
			}
			fmt.Fprint(d.Out, m.DBML)
			c := m.Schema.Counts()
			fmt.Fprintf(d.Err, "%s %d tables, %d relations\n", symbolOK, c.Tables, c.ForeignKeys)
			return nil
		}),
	}
	pull.Flags().String("db", "", "database (default: the connection's)")
	cmd.AddCommand(pull)
	return cmd
}

func loadModel(path string) (*schema.Database, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, errs.Wrap(errs.Invalid, "cannot read model", err)
	}
	db, err := dbml.Parse(string(src))
	if err != nil {
		return nil, err
	}
	for _, p := range dbml.Validate(db) {
		if p.Level == "error" {
			return nil, errs.Newf(errs.Invalid, "%s: %s", filepath.Base(path), p.Message)
		}
	}
	return db, nil
}

// --- diff -----------------------------------------------------------------

func diffCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <name[@env]> --model file.dbml",
		Short: "Changes between a model and the live database, as SQL with risks",
		Args:  cobra.ExactArgs(1),
		RunE: withApp(d, func(ctx context.Context, a *app.App, cmd *cobra.Command, args []string) error {
			it, err := resolve(ctx, a, args[0])
			if err != nil {
				return err
			}
			path, _ := cmd.Flags().GetString("model")
			target, err := loadModel(path)
			if err != nil {
				return err
			}
			diff, err := a.Designer.Diff(ctx, it.ID, dbFlag(cmd), target)
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return printJSON(d.Out, diff)
			}
			printHeader(d, it, dbFlag(cmd), path)
			if len(diff.Changes) == 0 {
				fmt.Fprintf(d.Out, "  %s no changes\n", symbolOK)
				return nil
			}
			for _, c := range diff.Changes {
				fmt.Fprintf(d.Out, "  + %s\n", c.Describe())
			}
			fmt.Fprintln(d.Out)
			plan(d.Out, diff.Plan)
			return nil
		}),
	}
	cmd.Flags().String("model", "", "DBML file")
	cmd.Flags().String("db", "", "database (default: the connection's)")
	_ = cmd.MarkFlagRequired("model")
	return cmd
}

func printHeader(d Deps, it *connections.Item, db, model string) {
	if db == "" {
		db = it.Database
	}
	fmt.Fprintf(d.Out, "%s %s · %s · %s\n", it.Name, envDot(d.Out, it.Environment), db, model)
}

// --- apply ----------------------------------------------------------------

func applyCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply <name[@env]> (--model file.dbml | --sql file.sql)",
		Short: "Review and run a migration from a model, or a SQL file, through policy and audit",
		Args:  cobra.ExactArgs(1),
		RunE: withApp(d, func(ctx context.Context, a *app.App, cmd *cobra.Command, args []string) error {
			it, err := resolve(ctx, a, args[0])
			if err != nil {
				return err
			}
			modelPath, _ := cmd.Flags().GetString("model")
			sqlPath, _ := cmd.Flags().GetString("sql")
			yes, _ := cmd.Flags().GetBool("yes")
			unlock, _ := cmd.Flags().GetBool("unlock")
			confirmation, _ := cmd.Flags().GetString("confirm")
			saveDir, _ := cmd.Flags().GetString("save-migration")
			database := dbFlag(cmd)
			if database == "" {
				database = it.Database
			}

			var p *changeset.Plan
			var decision policy.Decision
			switch {
			case modelPath != "":
				target, err := loadModel(modelPath)
				if err != nil {
					return err
				}
				diff, err := a.Designer.Diff(ctx, it.ID, database, target)
				if err != nil {
					return err
				}
				printHeader(d, it, database, modelPath)
				if len(diff.Changes) == 0 {
					fmt.Fprintf(d.Out, "  %s nothing to apply\n", symbolOK)
					return nil
				}
				p, decision = diff.Plan, diff.Decision
			case sqlPath != "":
				src, err := os.ReadFile(sqlPath)
				if err != nil {
					return errs.Wrap(errs.Invalid, "cannot read SQL file", err)
				}
				p = &changeset.Plan{Database: ident.Name(database), Statements: splitStatements(string(src))}
				printHeader(d, it, database, sqlPath)
				res, err := a.Changes.Apply(ctx, changes.ApplyInput{ConnectionID: it.ID, Database: database, Plan: p, DryRun: true, Actor: actorName(), Surface: surface})
				if err != nil {
					return err
				}
				decision = res.Decision
			default:
				return errs.New(errs.Invalid, "give --model file.dbml or --sql file.sql")
			}

			plan(d.Out, p)
			if unlock {
				if _, err := a.Connections.Unlock(ctx, it.ID, time.Hour); err != nil {
					return err
				}
			}
			if decision.Outcome == policy.Deny {
				return errs.New(errs.Forbidden, decision.Reason+" (re-run with --unlock)")
			}
			if !yes {
				fmt.Fprintf(d.Out, "\nApply %d statements to %s %s? [y/N] ", len(p.Statements), it.Name, envDot(d.Out, it.Environment))
				line, _ := bufio.NewReader(d.In).ReadString('\n')
				if l := strings.ToLower(strings.TrimSpace(line)); l != "y" && l != "yes" {
					fmt.Fprintln(d.Out, "  cancelled")
					return nil
				}
			}
			res, err := a.Changes.Apply(ctx, changes.ApplyInput{
				ConnectionID: it.ID, Database: database, Plan: p, Confirmation: confirmation,
				Transaction: true, Actor: actorName(), Surface: surface,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(d.Out, "%s applied in %d ms, one transaction\n", symbolOK, res.Duration.Milliseconds())
			if saveDir != "" {
				up, down, err := saveMigration(saveDir, p)
				if err != nil {
					return err
				}
				fmt.Fprintf(d.Out, "%s saved %s and %s\n", symbolOK, up, filepath.Base(down))
			}
			return nil
		}),
	}
	cmd.Flags().String("model", "", "DBML file to apply")
	cmd.Flags().String("sql", "", "SQL file to apply as is")
	cmd.Flags().String("db", "", "database (default: the connection's)")
	cmd.Flags().BoolP("yes", "y", false, "do not ask before applying")
	cmd.Flags().Bool("unlock", false, "unlock a read-only connection for this run")
	cmd.Flags().String("confirm", "", "the database name, when policy asks for it")
	cmd.Flags().String("save-migration", "", "directory to write <timestamp>_<name>.up.sql and .down.sql")
	return cmd
}

func actorName() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "cli"
}

// splitStatements cuts a SQL script on semicolons at line ends; good enough
// for migration files, which the person reviews before running.
func splitStatements(src string) []changeset.Statement {
	var out []changeset.Statement
	var buf strings.Builder
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
		if strings.HasSuffix(trimmed, ";") {
			s := strings.TrimSpace(buf.String())
			out = append(out, changeset.Statement{Kind: changeset.DDL, SQL: s, Describe: firstLine(s)})
			buf.Reset()
		}
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		out = append(out, changeset.Statement{Kind: changeset.DDL, SQL: s, Describe: firstLine(s)})
	}
	return out
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	if len(line) > 72 {
		return line[:72] + "…"
	}
	return line
}

func saveMigration(dir string, p *changeset.Plan) (string, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	name := strings.ToLower(strings.NewReplacer(" ", "_", ".", "_", "/", "_", "→", "to").Replace(p.Title()))
	if len(name) > 40 {
		name = name[:40]
	}
	stamp := time.Now().UTC().Format("20060102150405")
	up := filepath.Join(dir, stamp+"_"+name+".up.sql")
	down := filepath.Join(dir, stamp+"_"+name+".down.sql")
	if err := os.WriteFile(up, []byte(p.SQL()), 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(down, []byte(p.DownSQL()), 0o644); err != nil {
		return "", "", err
	}
	return up, down, nil
}

// --- audit ----------------------------------------------------------------

func auditCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit <name[@env]>",
		Short: "Writes Mesa ran on a connection, newest first",
		Args:  cobra.ExactArgs(1),
		RunE: withApp(d, func(ctx context.Context, a *app.App, cmd *cobra.Command, args []string) error {
			it, err := resolve(ctx, a, args[0])
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetInt("limit")
			list, err := a.Changes.Audit(ctx, it.ID, limit)
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return printJSON(d.Out, list)
			}
			rows := make([][]string, 0, len(list))
			for _, e := range list {
				sym := symbolOK
				if e.Status != "applied" {
					sym = symbolErr
				}
				rows = append(rows, []string{e.StartedAt.Local().Format("2006-01-02 15:04"), sym + " " + string(e.Status), e.Surface, e.Actor, e.Summary})
			}
			table(d.Out, []string{"when", "status", "surface", "actor", "change"}, rows, nil)
			return nil
		}),
	}
	cmd.Flags().Int("limit", 50, "entries")
	return cmd
}

// --- key ------------------------------------------------------------------

func keyCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "key",
		Short: "Generate a key for MESA_APP_KEY",
		RunE: func(_ *cobra.Command, _ []string) error {
			k, err := secret.GenerateKey()
			if err != nil {
				return err
			}
			fmt.Fprintln(d.Out, k)
			return nil
		},
	}
}

var _ = connection.Local
