package changeset

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactedStatementNeverShowsSecret(t *testing.T) {
	st := Statement{Kind: DDL, SQL: "CREATE ROLE r WITH LOGIN PASSWORD 'hunter2'", Redacted: "CREATE ROLE r WITH LOGIN PASSWORD '********'", Describe: "Create role r"}
	p := &Plan{Statements: []Statement{st}}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"json": string(b), "display": st.Display(), "script": p.SQL()} {
		if strings.Contains(text, "hunter2") {
			t.Errorf("%s leaks the password: %s", name, text)
		}
	}
	if !strings.Contains(st.SQL, "hunter2") {
		t.Error("execution SQL must keep the password")
	}
}
