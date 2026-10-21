package mysql

import (
	"strings"
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/core/value"
	"github.com/felipemalacarne/mesa/internal/engine"
)

func strp(s string) *string { return &s }

func shop() *schema.Database {
	return &schema.Database{Name: "shop", Schemas: []schema.Schema{{
		Name: "shop",
		Tables: []schema.Table{{
			Name: "orders",
			Columns: []schema.Column{
				{Name: "id", Type: "bigint", Identity: schema.IdentityByDefault},
				{Name: "status", Type: "varchar(20)", Default: strp("'new'"), Comment: "lifecycle"},
				{Name: "total", Type: "decimal(10,2)", Nullable: true},
			},
			PrimaryKey:  &schema.Key{Name: "PRIMARY", Columns: []ident.Name{"id"}},
			Indexes:     []schema.Index{{Name: "orders_status_idx", Columns: []ident.Name{"status"}}},
			RowEstimate: 50000,
		}},
	}}}
}

func compile(t *testing.T, current *schema.Database, cs *changeset.ChangeSet) *changeset.Plan {
	t.Helper()
	p, err := newDialect(current).compile(cs)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return p
}

func sqls(stmts []changeset.Statement) string {
	var out []string
	for _, s := range stmts {
		out = append(out, s.SQL)
	}
	return strings.Join(out, "\n")
}

func TestCreateTable(t *testing.T) {
	q := ident.Q("shop", "coupons")
	p := compile(t, nil, &changeset.ChangeSet{Schema: []schema.Change{{Kind: schema.CreateTable, Table: q, NewTable: &schema.Table{
		Name: "coupons",
		Columns: []schema.Column{
			{Name: "id", Type: "bigint", Identity: schema.IdentityByDefault},
			{Name: "code", Type: "varchar(40)"},
			{Name: "created_at", Type: "timestamptz", Default: strp("now()")},
			{Name: "token", Type: "uuid", Nullable: true, Comment: "it's secret"},
		},
		PrimaryKey: &schema.Key{Columns: []ident.Name{"id"}},
		Uniques:    []schema.Key{{Columns: []ident.Name{"code"}}},
		Comment:    "Discounts",
	}}}})
	want := "CREATE TABLE `shop`.`coupons` (\n" +
		"  `id` bigint NOT NULL AUTO_INCREMENT,\n" +
		"  `code` varchar(40) NOT NULL,\n" +
		"  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,\n" +
		"  `token` char(36) NULL COMMENT 'it''s secret',\n" +
		"  PRIMARY KEY (`id`),\n" +
		"  CONSTRAINT `coupons_code_key` UNIQUE (`code`)\n" +
		") COMMENT = 'Discounts'"
	if got := sqls(p.Statements); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if sqls(p.Down) != "DROP TABLE `shop`.`coupons`" {
		t.Errorf("down = %s", sqls(p.Down))
	}
	if !p.Has(changeset.RiskNoTransaction) {
		t.Error("DDL plans must say MySQL commits each statement")
	}
}

func TestColumnChangesRedefineTheWholeColumn(t *testing.T) {
	q := ident.Q("shop", "orders")
	cur := shop()
	newStatus := schema.Column{Name: "status", Type: "varchar(10)", Default: strp("'new'"), Comment: "lifecycle"}
	old := cur.Table(q).Columns[1]
	p := compile(t, cur, &changeset.ChangeSet{Schema: []schema.Change{
		{Kind: schema.AlterColumnType, Table: q, Column: &newStatus, OldColumn: &old},
		{Kind: schema.DropNotNull, Table: q, Name: "status"},
	}})
	want := "ALTER TABLE `shop`.`orders` MODIFY COLUMN `status` varchar(10) NOT NULL DEFAULT 'new' COMMENT 'lifecycle'\n" +
		"ALTER TABLE `shop`.`orders` MODIFY COLUMN `status` varchar(10) NULL DEFAULT 'new' COMMENT 'lifecycle'"
	if got := sqls(p.Statements); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	// Down runs in reverse: back to NOT NULL with the new type, then the old type.
	wantDown := "ALTER TABLE `shop`.`orders` MODIFY COLUMN `status` varchar(10) NOT NULL DEFAULT 'new' COMMENT 'lifecycle'\n" +
		"ALTER TABLE `shop`.`orders` MODIFY COLUMN `status` varchar(20) NOT NULL DEFAULT 'new' COMMENT 'lifecycle'"
	if got := sqls(p.Down); got != wantDown {
		t.Errorf("down got\n%s\nwant\n%s", got, wantDown)
	}
	if !p.Has(changeset.RiskDataLoss) {
		t.Errorf("narrowing varchar(20) to varchar(10) is data loss: %+v", p.Risks)
	}
	if p.Worst() != changeset.Danger {
		t.Errorf("worst = %v", p.Worst())
	}
}

func TestUnknownColumnIsRefused(t *testing.T) {
	_, err := newDialect(nil).compile(&changeset.ChangeSet{Schema: []schema.Change{{Kind: schema.SetNotNull, Table: ident.Q("shop", "orders"), Name: "x"}}})
	if !errs.Is(err, errs.Invalid) {
		t.Errorf("error = %v, want Invalid", err)
	}
}

func TestIndexesKeysAndForeignKeys(t *testing.T) {
	q := ident.Q("shop", "orders")
	p := compile(t, shop(), &changeset.ChangeSet{Schema: []schema.Change{
		{Kind: schema.DropIndex, Table: q, Name: "orders_status_idx"},
		{Kind: schema.AddForeignKey, Table: q, ForeignKey: &schema.ForeignKey{Columns: []ident.Name{"customer_id"}, References: ident.Q("shop", "customers"), RefColumns: []ident.Name{"id"}, OnDelete: "CASCADE"}},
		{Kind: schema.AddCheck, Table: q, Check: &schema.Check{Name: "total_positive", Expression: "total >= 0"}},
		{Kind: schema.SetTableComment, Table: q, Comment: "Customer orders"},
	}})
	want := "DROP INDEX `orders_status_idx` ON `shop`.`orders`\n" +
		"ALTER TABLE `shop`.`orders` ADD CONSTRAINT `orders_customer_id_fkey` FOREIGN KEY (`customer_id`) REFERENCES `shop`.`customers` (`id`) ON DELETE CASCADE\n" +
		"ALTER TABLE `shop`.`orders` ADD CONSTRAINT `total_positive` CHECK (total >= 0)\n" +
		"ALTER TABLE `shop`.`orders` COMMENT = 'Customer orders'"
	if got := sqls(p.Statements); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	wantDown := "ALTER TABLE `shop`.`orders` COMMENT = ''\n" +
		"ALTER TABLE `shop`.`orders` DROP CHECK `total_positive`\n" +
		"ALTER TABLE `shop`.`orders` DROP FOREIGN KEY `orders_customer_id_fkey`\n" +
		"CREATE INDEX `orders_status_idx` ON `shop`.`orders` (`status`)"
	if got := sqls(p.Down); got != wantDown {
		t.Errorf("down got\n%s\nwant\n%s", got, wantDown)
	}
	if _, err := newDialect(nil).compile(&changeset.ChangeSet{Schema: []schema.Change{{Kind: schema.CreateIndex, Table: q, Index: &schema.Index{Name: "p", Columns: []ident.Name{"id"}, Predicate: "id > 0"}}}}); !errs.Is(err, errs.Unsupported) {
		t.Errorf("partial index error = %v, want Unsupported", err)
	}
	if _, err := newDialect(nil).compile(&changeset.ChangeSet{Schema: []schema.Change{{Kind: schema.CreateSchema, Schema: "audit"}}}); !errs.Is(err, errs.Unsupported) {
		t.Errorf("create schema error = %v, want Unsupported", err)
	}
}

func TestRowChangesUseQuestionMarks(t *testing.T) {
	q := ident.Q("shop", "orders")
	p := compile(t, shop(), &changeset.ChangeSet{Rows: []changeset.RowChange{
		{Kind: changeset.Update, Table: q, Key: map[ident.Name]value.Value{"id": value.Int(7), "status": value.Text("new")}, Values: map[ident.Name]value.Value{"status": value.Text("paid")}},
		{Kind: changeset.Delete, Table: q, Key: map[ident.Name]value.Value{"id": value.Int(8), "total": value.Null()}},
	}})
	want := "UPDATE `shop`.`orders` SET `status` = ? WHERE `id` = ? AND `status` = ?\n" +
		"DELETE FROM `shop`.`orders` WHERE `id` = ? AND `total` IS NULL LIMIT 1"
	if got := sqls(p.Statements); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if p.Statements[0].ExpectRows != 1 || len(p.Statements[0].Args) != 3 {
		t.Errorf("update statement = %+v", p.Statements[0])
	}
	if p.Has(changeset.RiskNoTransaction) {
		t.Error("row changes alone run in one transaction")
	}
}

func TestSameTypeAndDefault(t *testing.T) {
	same := [][2]string{
		{"boolean", "tinyint(1)"}, {"int", "int(11)"}, {"integer", "int"}, {"numeric(10,2)", "decimal(10,2)"},
		{"decimal", "decimal(10,0)"}, {"bigint unsigned", "BIGINT UNSIGNED"}, {"timestamptz", "timestamp"},
		{"varchar", "varchar(255)"}, {"enum('a','b')", "enum('a','b')"}, {"double precision", "double"},
	}
	for _, p := range same {
		if !sameType(p[0], p[1]) {
			t.Errorf("sameType(%q, %q) = false", p[0], p[1])
		}
	}
	differ := [][2]string{{"tinyint", "tinyint(1)"}, {"int", "bigint"}, {"varchar(10)", "varchar(20)"}, {"int", "int unsigned"}, {"enum('a')", "enum('a','b')"}}
	for _, p := range differ {
		if sameType(p[0], p[1]) {
			t.Errorf("sameType(%q, %q) = true", p[0], p[1])
		}
	}
	defaults := [][2]string{{"true", "1"}, {"'5'", "5"}, {"now()", "CURRENT_TIMESTAMP"}, {"(uuid())", "uuid()"}, {"'new'", "_utf8mb4'new'"}, {"0.50", "0.5"}}
	for _, p := range defaults {
		if !sameDefault(p[0], p[1]) {
			t.Errorf("sameDefault(%q, %q) = false", p[0], p[1])
		}
	}
	if sameDefault("'new'", "'old'") {
		t.Error("different literals compare equal")
	}
}

func TestCatalogNormalization(t *testing.T) {
	cases := []struct{ def, typ, extra, want string }{
		{"new", "varchar", "", "'new'"},
		{"0", "int", "", "0"},
		{"CURRENT_TIMESTAMP", "timestamp", "default_generated", "CURRENT_TIMESTAMP"},
		{"uuid()", "char", "default_generated", "(uuid())"},
		{"it's", "text", "", "'it''s'"},
	}
	for _, c := range cases {
		if got := defaultSQL(c.def, c.typ, c.extra); got != c.want {
			t.Errorf("defaultSQL(%q) = %q, want %q", c.def, got, c.want)
		}
	}
	if got := checkExpression("(`total` >= 0)"); got != "total >= 0" {
		t.Errorf("checkExpression = %q", got)
	}
	if got := checkExpression("((`a` > 0) and (`b` = _utf8mb4'x'))"); got != "(a > 0) and (b = 'x')" {
		t.Errorf("checkExpression = %q", got)
	}
}

func TestReadOnlyStatements(t *testing.T) {
	for _, ok := range []string{"SELECT 1", "  -- note\nselect * from t", "/* x */ WITH a AS (SELECT 1) SELECT * FROM a", "show tables", "(SELECT 1) UNION (SELECT 2)", "EXPLAIN SELECT 1", "desc t"} {
		if err := readingStatement(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"DELETE FROM t", "SET SESSION transaction_read_only = OFF", "CALL p()", "select * from t into outfile '/tmp/x'", "EXPLAIN ANALYZE DELETE FROM t", "DROP TABLE t", "LOCK TABLES t WRITE"} {
		if err := readingStatement(bad); !errs.Is(err, errs.Forbidden) {
			t.Errorf("%q: error = %v, want Forbidden", bad, err)
		}
	}
	if got := statements("SELECT ';' AS a; SELECT `x;y` FROM t;\n\n"); len(got) != 2 {
		t.Errorf("statements = %q", got)
	}
}

func TestAccountNames(t *testing.T) {
	if got := account("app"); got != "'app'@'%'" {
		t.Errorf("account(app) = %s", got)
	}
	if got := account("app@10.0.0.%"); got != "'app'@'10.0.0.%'" {
		t.Errorf("account(app@host) = %s", got)
	}
	s := &Session{database: "shop"}
	p := s.CreateRolePlan(engineRole())
	if strings.Contains(p.Statements[0].Display(), "s3cret") || !strings.Contains(p.Statements[0].SQL, "s3cret") {
		t.Errorf("password must run but never show: %q / %q", p.Statements[0].SQL, p.Statements[0].Display())
	}
}

func engineRole() engine.RoleInput {
	return engine.RoleInput{Name: "reporter", Password: "s3cret", CanLogin: true, MemberOf: []ident.Name{"readers"}}
}
