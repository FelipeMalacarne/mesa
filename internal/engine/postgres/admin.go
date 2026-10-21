package postgres

import (
	"context"
	_ "embed"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/engine"
)

//go:embed sql/roles.sql
var rolesSQL string

// Roles implements engine.Admin.
func (s *Session) Roles(ctx context.Context) ([]engine.Role, error) {
	rows, err := s.pool.Query(ctx, rolesSQL)
	if err != nil {
		return nil, wrap(err, "cannot list roles")
	}
	defer rows.Close()
	var out []engine.Role
	for rows.Next() {
		var r engine.Role
		var name string
		var validUntil pgtype.Timestamptz
		if err := rows.Scan(&name, &r.Superuser, &r.CanLogin, &r.CreateDB, &r.CreateRole, &r.Inherit,
			&r.ConnLimit, &validUntil, &r.MemberOf); err != nil {
			return nil, wrap(err, "cannot list roles")
		}
		r.Name = ident.Name(name)
		if validUntil.Valid && validUntil.InfinityModifier == pgtype.Finite {
			t := validUntil.Time
			r.ValidUntil = &t
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap(err, "cannot list roles")
	}
	return out, nil
}

// CreateDatabasePlan implements engine.Admin. CREATE DATABASE cannot run
// inside a transaction block; the description says so for the review.
func (s *Session) CreateDatabasePlan(name, owner ident.Name) *changeset.Plan {
	sql := "CREATE DATABASE " + name.Quote()
	if owner != "" {
		sql += " OWNER " + owner.Quote()
	}
	return &changeset.Plan{
		Database: ident.Name(s.database),
		Statements: []changeset.Statement{
			ddl(sql, "Create database "+name.String()+" (runs outside a transaction)"),
		},
		Down: []changeset.Statement{
			ddl("DROP DATABASE "+name.Quote(), "Drop database "+name.String()),
		},
		Summary: changeset.Summary{SchemaChanges: 1},
	}
}

// CreateRolePlan implements engine.Admin. The password cannot be bound as a
// parameter, so it is embedded as a quoted literal; the description never
// shows it.
func (s *Session) CreateRolePlan(in engine.RoleInput) *changeset.Plan {
	opts := []string{boolOption(in.CanLogin, "LOGIN", "NOLOGIN")}
	if in.Superuser {
		opts = append(opts, "SUPERUSER")
	}
	if in.CreateDB {
		opts = append(opts, "CREATEDB")
	}
	if in.CreateRole {
		opts = append(opts, "CREATEROLE")
	}
	if in.ConnLimit > 0 || in.ConnLimit == -1 {
		opts = append(opts, "CONNECTION LIMIT "+strconv.Itoa(in.ConnLimit))
	}
	create := ddl("CREATE ROLE "+in.Name.Quote()+" WITH "+strings.Join(opts, " "), "Create role "+in.Name.String())
	if in.Password != "" {
		shown := create.SQL + " PASSWORD '********'"
		create.SQL += " PASSWORD " + quoteLiteral(in.Password)
		create.Redacted = shown
	}
	stmts := []changeset.Statement{create}
	for _, g := range in.MemberOf {
		stmts = append(stmts, ddl("GRANT "+g.Quote()+" TO "+in.Name.Quote(), "Add "+in.Name.String()+" to "+g.String()))
	}
	return &changeset.Plan{
		Database:   ident.Name(s.database),
		Statements: stmts,
		Down:       []changeset.Statement{ddl("DROP ROLE "+in.Name.Quote(), "Drop role "+in.Name.String())},
		Summary:    changeset.Summary{SchemaChanges: len(stmts)},
	}
}

// DropRolePlan implements engine.Admin.
func (s *Session) DropRolePlan(name ident.Name) *changeset.Plan {
	return &changeset.Plan{
		Database:   ident.Name(s.database),
		Statements: []changeset.Statement{ddl("DROP ROLE "+name.Quote(), "Drop role "+name.String())},
		Risks: []changeset.Risk{{
			Level:   changeset.Warning,
			Code:    changeset.RiskIrreversible,
			Message: "the role's password and memberships cannot be restored",
			Object:  name.String(),
		}},
		Summary: changeset.Summary{SchemaChanges: 1},
	}
}

func boolOption(on bool, yes, no string) string {
	if on {
		return yes
	}
	return no
}

// ddl builds a DDL statement without arguments.
func ddl(sql, describe string) changeset.Statement {
	return changeset.Statement{Kind: changeset.DDL, SQL: sql, Describe: describe}
}

// quoteLiteral renders s as a SQL string literal. Backslashes are literal
// under standard_conforming_strings, the default since PostgreSQL 9.1.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
