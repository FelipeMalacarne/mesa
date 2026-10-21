package changeset

import (
	"reflect"
	"testing"
)

func TestSplitScript(t *testing.T) {
	src := `-- header
create table a (x text default 'a;b');
insert into "we;ird" values ($$ ; $$);
create function f() returns int as $body$ begin return 1; end $body$ language plpgsql;
/* block ; comment */ select 1;
;;
-- only a comment;
`
	got := SplitScript(src)
	want := []string{
		"-- header\ncreate table a (x text default 'a;b')",
		`insert into "we;ird" values ($$ ; $$)`,
		"create function f() returns int as $body$ begin return 1; end $body$ language plpgsql",
		"/* block ; comment */ select 1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitScript:\n got %q\nwant %q", got, want)
	}
}

func TestFromScriptRisks(t *testing.T) {
	cases := []struct {
		sql  string
		code RiskCode
		lvl  RiskLevel
	}{
		{"DROP TABLE shipments", RiskDataLoss, Danger},
		{"truncate orders", RiskDataLoss, Danger},
		{"ALTER TABLE customers DROP COLUMN phone", RiskDataLoss, Danger},
		{"alter table customers drop phone", RiskDataLoss, Danger},
		{"alter table customers drop column city", RiskDataLoss, Danger},
		{"alter table customers drop column if exists city", RiskDataLoss, Danger},
		{"alter table t alter column a drop default, drop column c", RiskDataLoss, Danger},
		{"delete from orders", RiskManyRows, Danger},
		{"update orders set total = 0", RiskManyRows, Danger},
		{"alter table orders alter column total type numeric(12,2)", RiskTableRewrite, Warning},
		{"create index on orders (placed_at)", RiskExclusiveLock, Info},
	}
	for _, c := range cases {
		p := FromScript("db", c.sql)
		if !p.Has(c.code) {
			t.Errorf("%q: want risk %s, got %+v", c.sql, c.code, p.Risks)
			continue
		}
		for _, r := range p.Risks {
			if r.Code == c.code && r.Level != c.lvl {
				t.Errorf("%q: want level %s, got %s", c.sql, c.lvl, r.Level)
			}
		}
	}
	for _, safe := range []string{
		"delete from orders where id = 1",
		"update orders set total = 0 where id = 2",
		"alter table customers drop constraint customers_email_key",
		"alter table customers alter column name drop not null",
		"alter table customers alter column name drop default",
		"alter table customers alter column id drop identity",
		"create index concurrently orders_x on orders (x)",
	} {
		p := FromScript("db", safe)
		if p.Worst() == Danger || p.Has(RiskDataLoss) || p.Has(RiskManyRows) {
			t.Errorf("%q: unexpected risks %+v", safe, p.Risks)
		}
	}
	if p := FromScript("db", "update t set a = 1 where b = 2; insert into t values (1)"); p.Summary.Updated != 2 || p.Statements[0].Kind != DML {
		t.Errorf("DML classification: %+v", p)
	}
}
