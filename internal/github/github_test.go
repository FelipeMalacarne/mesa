package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/felipemalacarne/mesa/internal/app/provision"
	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// fake is a GitHub with one repository: commits are whole file maps.
type fake struct {
	mu       sync.Mutex
	token    string
	refs     map[string]string            // branch → commit
	commits  map[string]map[string]string // commit → path → content
	trees    map[string]map[string]string // tree → path → content
	pulls    []map[string]any
	moveNext bool // the next ref update finds the branch moved
	n        int
}

func newFake(t *testing.T) (*fake, *Client) {
	t.Helper()
	f := &fake{
		token: "t0ken",
		refs:  map[string]string{"main": "c0"},
		commits: map[string]map[string]string{"c0": {
			".sops.yaml":                          "creation_rules: []\n",
			"clusters/saradomin/mesa/apps/a.yaml": "a\n",
		}},
		trees: map[string]map[string]string{},
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, New("t0ken", srv.URL)
}

func (f *fake) id(prefix string) string { f.n++; return fmt.Sprintf("%s%d", prefix, f.n) }

func (f *fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+f.token {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
		return
	}
	const base = "/repos/felipe/gielinor"
	p := strings.TrimPrefix(r.URL.Path, base)
	reply := func(code int, v any) {
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	switch {
	case !strings.HasPrefix(r.URL.Path, base):
		reply(404, map[string]string{"message": "Not Found"})
	case r.Method == "GET" && strings.HasPrefix(p, "/contents/"):
		files := f.commits[f.refs[r.URL.Query().Get("ref")]]
		want := strings.TrimPrefix(p, "/contents/")
		if c, ok := files[want]; ok {
			reply(200, map[string]string{"type": "file", "name": want, "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(c))})
			return
		}
		var list []map[string]string
		seen := map[string]bool{}
		for path := range files {
			if rest, ok := strings.CutPrefix(path, want+"/"); ok {
				name, _, _ := strings.Cut(rest, "/")
				if !seen[name] {
					seen[name] = true
					list = append(list, map[string]string{"name": name})
				}
			}
		}
		if list == nil {
			reply(404, map[string]string{"message": "Not Found"})
			return
		}
		reply(200, list)
	case r.Method == "GET" && strings.HasPrefix(p, "/git/ref/heads/"):
		reply(200, map[string]any{"object": map[string]string{"sha": f.refs[strings.TrimPrefix(p, "/git/ref/heads/")]}})
	case r.Method == "GET" && strings.HasPrefix(p, "/git/commits/"):
		reply(200, map[string]any{"tree": map[string]string{"sha": "tree-of-" + strings.TrimPrefix(p, "/git/commits/")}})
	case r.Method == "POST" && p == "/git/trees":
		baseFiles := f.commits[strings.TrimPrefix(in["base_tree"].(string), "tree-of-")]
		files := map[string]string{}
		for k, v := range baseFiles {
			files[k] = v
		}
		for _, e := range in["tree"].([]any) {
			e := e.(map[string]any)
			files[e["path"].(string)] = e["content"].(string)
		}
		id := f.id("t")
		f.trees[id] = files
		reply(201, map[string]string{"sha": id})
	case r.Method == "POST" && p == "/git/commits":
		id := f.id("c")
		f.commits[id] = f.trees[in["tree"].(string)]
		reply(201, map[string]string{"sha": id, "html_url": "https://github.com/felipe/gielinor/commit/" + id})
	case r.Method == "PATCH" && strings.HasPrefix(p, "/git/refs/heads/"):
		if f.moveNext {
			f.moveNext = false
			reply(422, map[string]string{"message": "Update is not a fast forward"})
			return
		}
		f.refs[strings.TrimPrefix(p, "/git/refs/heads/")] = in["sha"].(string)
		reply(200, map[string]any{})
	case r.Method == "POST" && p == "/git/refs":
		f.refs[strings.TrimPrefix(in["ref"].(string), "refs/heads/")] = in["sha"].(string)
		reply(201, map[string]any{})
	case r.Method == "POST" && p == "/pulls":
		f.pulls = append(f.pulls, in)
		reply(201, map[string]string{"html_url": "https://github.com/felipe/gielinor/pull/9"})
	default:
		reply(404, map[string]string{"message": "Not Found"})
	}
}

func TestReadListCommit(t *testing.T) {
	f, c := newFake(t)
	ctx := context.Background()
	r, err := c.Repo("felipe/gielinor", "main")
	if err != nil {
		t.Fatal(err)
	}
	if b, err := r.Read(ctx, ".sops.yaml"); err != nil || string(b) != "creation_rules: []\n" {
		t.Fatalf("Read = %q %v", b, err)
	}
	if _, err := r.Read(ctx, "nope.yaml"); !errs.Is(err, errs.NotFound) {
		t.Fatalf("Read of a missing file = %v", err)
	}
	if names, err := r.List(ctx, "clusters/saradomin/mesa/apps"); err != nil || len(names) != 1 || names[0] != "a.yaml" {
		t.Fatalf("List = %v %v", names, err)
	}
	if names, err := r.List(ctx, "clusters/zamorak/mesa"); err != nil || len(names) != 0 {
		t.Fatalf("List of a missing dir = %v %v", names, err)
	}

	done, err := r.Commit(ctx, provision.Commit{Message: "mesa: shop", Files: map[string][]byte{
		"clusters/saradomin/mesa/apps/shop.yaml":     []byte("shop\n"),
		"clusters/saradomin/mesa/kustomization.yaml": []byte("k\n"),
	}})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	head := f.commits[f.refs["main"]]
	if f.refs["main"] != done.SHA || head["clusters/saradomin/mesa/apps/shop.yaml"] != "shop\n" || head[".sops.yaml"] == "" {
		t.Fatalf("main = %s, files %v", f.refs["main"], keys(head))
	}
	if !strings.Contains(done.URL, done.SHA) || done.PullRequestURL != "" {
		t.Errorf("committed = %+v", done)
	}

	// Someone pushed meanwhile: Conflict, and main is untouched.
	f.moveNext = true
	before := f.refs["main"]
	if _, err := r.Commit(ctx, provision.Commit{Message: "x", Files: map[string][]byte{"x": []byte("x")}}); !errs.Is(err, errs.Conflict) {
		t.Fatalf("moved branch = %v", err)
	}
	if f.refs["main"] != before {
		t.Fatal("a refused update moved main")
	}

	// A pull request leaves main alone.
	done, err = r.Commit(ctx, provision.Commit{
		Message: "mesa: blog", Files: map[string][]byte{"b.yaml": []byte("b\n")},
		PullRequest: &provision.PullRequest{Branch: "mesa/blog", Title: "Database blog", Body: "From Mesa"},
	})
	if err != nil || done.PullRequestURL == "" {
		t.Fatalf("pull request: %+v %v", done, err)
	}
	if f.refs["main"] != before || f.commits[f.refs["mesa/blog"]]["b.yaml"] != "b\n" {
		t.Fatalf("refs after PR = %v", f.refs)
	}
	if len(f.pulls) != 1 || f.pulls[0]["base"] != "main" || f.pulls[0]["head"] != "mesa/blog" {
		t.Fatalf("pulls = %v", f.pulls)
	}
}

func TestErrors(t *testing.T) {
	f, _ := newFake(t)
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	defer srv.Close()
	bad := New("wrong", srv.URL)
	r, _ := bad.Repo("felipe/gielinor", "main")
	if _, err := r.Read(context.Background(), ".sops.yaml"); !errs.Is(err, errs.Forbidden) {
		t.Fatalf("bad token = %v", err)
	}
	if _, err := bad.Repo("gielinor", "main"); !errs.Is(err, errs.Invalid) {
		t.Fatalf("repo without owner = %v", err)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
