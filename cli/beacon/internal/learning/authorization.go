package learning

// Project authorization, shared by the storage lifecycle and the MCP surface so
// there is exactly one authorization mechanism. Authority is always server- or
// operator-supplied; a caller-supplied project identifier is never proof.

import (
	"context"
	"errors"
	"strings"
)

// ProjectAuthorizer decides whether a caller may act on a project.
type ProjectAuthorizer interface {
	AuthorizeProject(ctx context.Context, projectID string) error
}

// ErrProjectNotAuthorized is returned when a caller may not act on a project.
var ErrProjectNotAuthorized = errors.New("project not authorized")

// AllowAllProjects is the trusted-operator policy: the caller is the local
// operator who owns the store, so every project is reachable. It must be chosen
// explicitly and never used to authenticate an untrusted caller.
type AllowAllProjects struct{}

func (AllowAllProjects) AuthorizeProject(context.Context, string) error { return nil }

// ProjectAllowlist authorizes only a fixed set of project IDs. A nil or empty
// allowlist denies everything (fail-closed).
type ProjectAllowlist struct {
	allowed map[string]struct{}
}

// NewProjectAllowlist builds an allowlist from project IDs. Blank entries are
// ignored; an allowlist built from no IDs authorizes nothing.
func NewProjectAllowlist(ids ...string) *ProjectAllowlist {
	allowlist := &ProjectAllowlist{allowed: make(map[string]struct{}, len(ids))}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			allowlist.allowed[id] = struct{}{}
		}
	}
	return allowlist
}

// AuthorizeProject allows only listed project IDs. It is fail-closed.
func (a *ProjectAllowlist) AuthorizeProject(_ context.Context, projectID string) error {
	if a == nil || len(a.allowed) == 0 {
		return ErrProjectNotAuthorized
	}
	if _, ok := a.allowed[strings.TrimSpace(projectID)]; !ok {
		return ErrProjectNotAuthorized
	}
	return nil
}

// SingleProject returns the one authorized project when the allowlist holds
// exactly one, so a caller can push the scope into a SQL predicate.
func (a *ProjectAllowlist) SingleProject() (string, bool) {
	if a == nil || len(a.allowed) != 1 {
		return "", false
	}
	for id := range a.allowed {
		return id, true
	}
	return "", false
}
