package provision

import "context"

// Repo is the Git repository a cluster is reconciled from (Argo CD, Flux).
// Mesa writes what it provisions there, so the repository and the cluster
// stay the same.
type Repo interface {
	// Read returns a file at the head of the branch; errs.NotFound when it
	// is missing.
	Read(ctx context.Context, path string) ([]byte, error)
	// List returns the names in a directory; none when it is missing.
	List(ctx context.Context, dir string) ([]string, error)
	// Commit writes files in one commit, on the branch itself or, with a
	// pull request, on a new branch. errs.Conflict when the branch moved
	// meanwhile: read again and retry.
	Commit(ctx context.Context, c Commit) (*Committed, error)
}

// Commit is one commit of whole files.
type Commit struct {
	Message string
	Files   map[string][]byte
	// PullRequest, when set, puts the commit on a new branch and opens a
	// pull request into the repository's branch.
	PullRequest *PullRequest
}

// PullRequest describes the pull request a commit opens.
type PullRequest struct {
	Branch string
	Title  string
	Body   string
}

// Committed is where the commit landed.
type Committed struct {
	SHA            string
	URL            string
	PullRequestURL string
}

// RepoFactory opens a repository ("owner/name") on a branch.
type RepoFactory func(repo, branch string) (Repo, error)
