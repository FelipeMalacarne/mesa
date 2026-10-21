package mysql

import (
	"context"
	_ "embed"
	"strconv"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/engine"
)

//go:embed sql/roles.sql
var rolesSQL string

// Roles implements engine.Admin. MySQL accounts are user@host; an account
// for any host ('%') is shown by its user name alone.
func (s *Session) Roles(ctx context.Context) ([]engine.Role, error) {
	rows, err := s.db.QueryContext(ctx, rolesSQL)
	if err != nil {
		return nil, wrap(err, "cannot list accounts")
	}
	defer func() { _ = rows.Close() }()
	var out []engine.Role
	for rows.Next() {
		var r engine.Role
		var user, host, memberOf string
		if err := rows.Scan(&user, &host, &r.Superuser, &r.CanLogin, &r.CreateDB, &r.CreateRole, &r.ConnLimit, &memberOf); err != nil {
			return nil, wrap(err, "cannot list accounts")
		}
		r.Name = ident.Name(accountName(user, host))
		r.Inherit = true
		if r.ConnLimit == 0 {
			r.ConnLimit = -1
		}
		if memberOf != "" {
			r.MemberOf = strings.Split(memberOf, "\n")
		}
		out = append(out, r)
	}
	return out, wrap(rows.Err(), "cannot list accounts")
}

func accountName(user, host string) string {
	if host == "%" {
		return user
	}
	return user + "@" + host
}

// account renders 'user'@'host' from "user" or "user@host".
func account(name ident.Name) string {
	user, host := string(name), "%"
	if i := strings.LastIndexByte(user, '@'); i > 0 {
		user, host = user[:i], user[i+1:]
	}
	return quoteLiteral(user) + "@" + quoteLiteral(host)
}

// CreateDatabasePlan implements engine.Admin. MySQL databases have no
// owner; grant privileges on it separately.
func (s *Session) CreateDatabasePlan(name, _ ident.Name) *changeset.Plan {
	return &changeset.Plan{
		Database:   ident.Name(s.database),
		Statements: []changeset.Statement{ddl("CREATE DATABASE "+quote(name), "Create database "+name.String())},
		Down:       []changeset.Statement{ddl("DROP DATABASE "+quote(name), "Drop database "+name.String())},
		Summary:    changeset.Summary{SchemaChanges: 1},
	}
}

// CreateRolePlan implements engine.Admin. The password is embedded as a
// literal; the redacted form is what review and audit show.
func (s *Session) CreateRolePlan(in engine.RoleInput) *changeset.Plan {
	acct := account(in.Name)
	sql := "CREATE USER " + acct
	shown := sql
	if in.Password != "" {
		sql += " IDENTIFIED BY " + quoteLiteral(in.Password)
		shown += " IDENTIFIED BY '********'"
	}
	var tail string
	if in.ConnLimit > 0 {
		tail += " WITH MAX_USER_CONNECTIONS " + strconv.Itoa(in.ConnLimit)
	}
	if !in.CanLogin {
		tail += " ACCOUNT LOCK"
	}
	create := ddl(sql+tail, "Create account "+in.Name.String())
	if in.Password != "" {
		create.Redacted = shown + tail
	}
	stmts := []changeset.Statement{create}
	if in.Superuser {
		stmts = append(stmts, ddl("GRANT ALL PRIVILEGES ON *.* TO "+acct+" WITH GRANT OPTION", "Make "+in.Name.String()+" a superuser"))
	} else {
		if in.CreateDB {
			stmts = append(stmts, ddl("GRANT CREATE ON *.* TO "+acct, "Let "+in.Name.String()+" create databases"))
		}
		if in.CreateRole {
			stmts = append(stmts, ddl("GRANT CREATE USER ON *.* TO "+acct, "Let "+in.Name.String()+" create accounts"))
		}
	}
	for _, g := range in.MemberOf {
		stmts = append(stmts, ddl("GRANT "+account(g)+" TO "+acct, "Add "+in.Name.String()+" to "+g.String()))
	}
	return &changeset.Plan{
		Database:   ident.Name(s.database),
		Statements: stmts,
		Down:       []changeset.Statement{ddl("DROP USER "+acct, "Drop account "+in.Name.String())},
		Summary:    changeset.Summary{SchemaChanges: len(stmts)},
	}
}

// DropRolePlan implements engine.Admin.
func (s *Session) DropRolePlan(name ident.Name) *changeset.Plan {
	return &changeset.Plan{
		Database:   ident.Name(s.database),
		Statements: []changeset.Statement{ddl("DROP USER "+account(name), "Drop account "+name.String())},
		Risks: []changeset.Risk{{
			Level:   changeset.Warning,
			Code:    changeset.RiskIrreversible,
			Message: "the account's password and grants cannot be restored",
			Object:  name.String(),
		}},
		Summary: changeset.Summary{SchemaChanges: 1},
	}
}
