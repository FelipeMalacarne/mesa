package desktop

import "testing"

func TestRoute(t *testing.T) {
	ok := map[string]string{
		"mesa://":  "/",
		"mesa:///": "/",
		"MESA://c/0190a3c4-7d38-7ae0-a380-a1ffd17ceeb9/sql":                        "/c/0190a3c4-7d38-7ae0-a380-a1ffd17ceeb9/sql",
		"mesa://c/0190a3c4-7d38-7ae0-a380-a1ffd17ceeb9/db/shop/t/public/orders":    "/c/0190a3c4-7d38-7ae0-a380-a1ffd17ceeb9/db/shop/t/public/orders",
		"mesa://c/0190a3c4-7d38-7ae0-a380-a1ffd17ceeb9/monitor?database=shop":      "/c/0190a3c4-7d38-7ae0-a380-a1ffd17ceeb9/monitor?database=shop",
		"mesa://c/0190a3c4-7d38-7ae0-a380-a1ffd17ceeb9/db/my%20db/t/public/orders": "/c/0190a3c4-7d38-7ae0-a380-a1ffd17ceeb9/db/my%20db/t/public/orders",
	}
	for in, want := range ok {
		got, valid := Route(in)
		if !valid || got != want {
			t.Errorf("Route(%q) = %q, %v; want %q", in, got, valid, want)
		}
	}
	for _, bad := range []string{
		"https://evil.example/",
		"mesa://c/x/../../etc",
		"mesa://user:pw@c/x",
		"mesa://c/x?next=javascript:alert(1)",
		"mesa://c/x#frag",
		"mesa://c/x\"onload=",
		"mesa:javascript:alert(1)",
	} {
		if got, valid := Route(bad); valid {
			t.Errorf("Route(%q) = %q, want refused", bad, got)
		}
	}
}

func TestFromArgs(t *testing.T) {
	if r, ok := FromArgs([]string{"--flag", "mesa://c/abc/sql"}); !ok || r != "/c/abc/sql" {
		t.Errorf("FromArgs = %q, %v", r, ok)
	}
	if _, ok := FromArgs([]string{"--flag", "file.txt"}); ok {
		t.Error("FromArgs found a link where there is none")
	}
}
