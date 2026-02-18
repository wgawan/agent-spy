package git

import (
	gogit "github.com/go-git/go-git/v5"
)

type Repo struct {
	repo *gogit.Repository
	path string
}

func Open(path string) (*Repo, error) {
	r := &Repo{path: path}
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		// Not a git repo - that's fine, gracefully degrade
		return r, nil
	}
	r.repo = repo
	return r, nil
}

func (r *Repo) Available() bool {
	return r.repo != nil
}

func (r *Repo) Branch() string {
	if r.repo == nil {
		return ""
	}
	ref, err := r.repo.Head()
	if err != nil {
		return ""
	}
	if ref.Name().IsBranch() {
		return ref.Name().Short()
	}
	// Detached HEAD - return short hash
	return ref.Hash().String()[:7]
}

func (r *Repo) Path() string {
	return r.path
}
