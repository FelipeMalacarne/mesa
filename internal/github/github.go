// Package github reads and writes a GitHub repository through the REST API,
// for Mesa to commit what it provisions to the repository a cluster is
// reconciled from. It implements the repository port of app/provision.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/app/provision"
	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// DefaultAPI is github.com's API; GitHub Enterprise Server has its own.
const DefaultAPI = "https://api.github.com"

// Client talks to the GitHub API with one token: a fine-grained token with
// read and write access to the repository's contents (and pull requests,
// to open them).
type Client struct {
	api   string
	token string
	http  *http.Client
}

// New returns a client for api (DefaultAPI when empty).
func New(token, api string) *Client {
	if api == "" {
		api = DefaultAPI
	}
	return &Client{api: strings.TrimRight(api, "/"), token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

// Repo opens "owner/name" on branch.
func (c *Client) Repo(fullName, branch string) (provision.Repo, error) {
	owner, name, ok := strings.Cut(fullName, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return nil, errs.Newf(errs.Invalid, "repository %q is not owner/name", fullName)
	}
	if branch == "" {
		return nil, errs.New(errs.Invalid, "the repository needs a branch")
	}
	return &repo{c: c, base: "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name), full: fullName, branch: branch}, nil
}

type repo struct {
	c      *Client
	base   string
	full   string
	branch string
}

type content struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func (r *repo) Read(ctx context.Context, p string) ([]byte, error) {
	var f content
	if err := r.c.do(ctx, http.MethodGet, r.contents(p), nil, &f); err != nil {
		return nil, err
	}
	if f.Type != "file" {
		return nil, errs.Newf(errs.Invalid, "%s is not a file", p)
	}
	if f.Encoding != "base64" {
		return nil, errs.Newf(errs.Unsupported, "%s is too large to read through the API", p)
	}
	return base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
}

func (r *repo) List(ctx context.Context, dir string) ([]string, error) {
	var entries []content
	err := r.c.do(ctx, http.MethodGet, r.contents(dir), nil, &entries)
	if errs.Is(err, errs.NotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out, nil
}

func (r *repo) contents(p string) string {
	var parts []string
	for _, s := range strings.Split(strings.Trim(path.Clean("/"+p), "/"), "/") {
		parts = append(parts, url.PathEscape(s))
	}
	return r.base + "/contents/" + strings.Join(parts, "/") + "?ref=" + url.QueryEscape(r.branch)
}

func (r *repo) Commit(ctx context.Context, cm provision.Commit) (*provision.Committed, error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := r.c.do(ctx, http.MethodGet, r.base+"/git/ref/heads/"+url.PathEscape(r.branch), nil, &ref); err != nil {
		return nil, err
	}
	var head struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := r.c.do(ctx, http.MethodGet, r.base+"/git/commits/"+ref.Object.SHA, nil, &head); err != nil {
		return nil, err
	}
	type entry struct {
		Path    string `json:"path"`
		Mode    string `json:"mode"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	paths := make([]string, 0, len(cm.Files))
	for p := range cm.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	entries := make([]entry, 0, len(paths))
	for _, p := range paths {
		entries = append(entries, entry{Path: p, Mode: "100644", Type: "blob", Content: string(cm.Files[p])})
	}
	var tree struct {
		SHA string `json:"sha"`
	}
	if err := r.c.do(ctx, http.MethodPost, r.base+"/git/trees", map[string]any{"base_tree": head.Tree.SHA, "tree": entries}, &tree); err != nil {
		return nil, err
	}
	var commit struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
	}
	body := map[string]any{"message": cm.Message, "tree": tree.SHA, "parents": []string{ref.Object.SHA}}
	if err := r.c.do(ctx, http.MethodPost, r.base+"/git/commits", body, &commit); err != nil {
		return nil, err
	}
	out := &provision.Committed{SHA: commit.SHA, URL: commit.HTMLURL}
	if cm.PullRequest == nil {
		// Fast-forward only: if someone pushed meanwhile, GitHub refuses and
		// the caller reads again.
		err := r.c.do(ctx, http.MethodPatch, r.base+"/git/refs/heads/"+url.PathEscape(r.branch), map[string]any{"sha": commit.SHA, "force": false}, nil)
		if errs.Is(err, errs.Invalid) {
			return nil, errs.Wrap(errs.Conflict, r.branch+" moved while Mesa was writing; try again", err)
		}
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	pr := cm.PullRequest
	if err := r.c.do(ctx, http.MethodPost, r.base+"/git/refs", map[string]any{"ref": "refs/heads/" + pr.Branch, "sha": commit.SHA}, nil); err != nil {
		return nil, err
	}
	var pull struct {
		HTMLURL string `json:"html_url"`
	}
	if err := r.c.do(ctx, http.MethodPost, r.base+"/pulls", map[string]any{"title": pr.Title, "head": pr.Branch, "base": r.branch, "body": pr.Body}, &pull); err != nil {
		return nil, err
	}
	out.PullRequestURL = pull.HTMLURL
	return out, nil
}

// do sends one API request, mapping GitHub's answers to error kinds.
func (c *Client) do(ctx context.Context, method, p string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.api+p, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return errs.Wrap(errs.Unavailable, "cannot reach GitHub", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&e)
		cause := fmt.Errorf("%s %s: %d %s", method, p, res.StatusCode, e.Message)
		switch res.StatusCode {
		case http.StatusNotFound:
			return errs.Wrap(errs.NotFound, "not found on GitHub (or the token cannot see it)", cause)
		case http.StatusUnauthorized, http.StatusForbidden:
			return errs.Wrap(errs.Forbidden, "GitHub refused the token: it needs read and write access to the repository's contents (and pull requests)", cause)
		case http.StatusConflict, http.StatusUnprocessableEntity:
			return errs.Wrap(errs.Invalid, "GitHub refused the change: "+e.Message, cause)
		}
		return errs.Wrap(errs.Unavailable, "GitHub answered "+res.Status, cause)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return errs.Wrap(errs.Unavailable, "unreadable answer from GitHub", err)
	}
	return nil
}
