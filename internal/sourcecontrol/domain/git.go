// Package domain owns provider-neutral source-control observations.
package domain

import "time"

// ChangeCounts summarizes porcelain categories without exposing repository content.
type ChangeCounts struct {
	Staged     int `json:"staged"`
	Modified   int `json:"modified"`
	Untracked  int `json:"untracked"`
	Conflicted int `json:"conflicted"`
}

// Remote is one configured Git transport URL.
type Remote struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Kind string `json:"kind"`
}

// Commit is bounded metadata for the current tip.
type Commit struct {
	Hash        string    `json:"hash"`
	ShortHash   string    `json:"shortHash"`
	Author      string    `json:"author"`
	Subject     string    `json:"subject"`
	CommittedAt time.Time `json:"committedAt"`
}

// Worktree is one Git-managed checkout.
type Worktree struct {
	Path     string `json:"path"`
	Head     string `json:"head"`
	Branch   string `json:"branch,omitempty"`
	Detached bool   `json:"detached"`
	Bare     bool   `json:"bare"`
	Locked   bool   `json:"locked"`
}

// State is a fresh, read-only Git snapshot.
type State struct {
	ProjectID      string       `json:"projectId"`
	Repository     bool         `json:"repository"`
	Branch         string       `json:"branch,omitempty"`
	Detached       bool         `json:"detached"`
	Head           string       `json:"head,omitempty"`
	Upstream       string       `json:"upstream,omitempty"`
	Ahead          int          `json:"ahead"`
	Behind         int          `json:"behind"`
	Changes        ChangeCounts `json:"changes"`
	Stashes        int          `json:"stashes"`
	OperationState string       `json:"operationState,omitempty"`
	LastCommit     *Commit      `json:"lastCommit,omitempty"`
	Remotes        []Remote     `json:"remotes"`
	Worktrees      []Worktree   `json:"worktrees"`
	ObservedAt     time.Time    `json:"observedAt"`
}
