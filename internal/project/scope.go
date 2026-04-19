package project

// scope.go — re-exports the scope-routing API from internal/scope so
// callers that already depend on the project package (e.g. internal/cli)
// can use the familiar project.ScopeFor form without importing the lower-
// level package directly.
//
// The implementation lives in internal/scope to break the import cycle
// that would otherwise form between project and its downstream callers
// (internal/initv2, internal/auth).

import "github.com/eddiecarpenter/gh-agentic/internal/scope"

// Scope flag constants re-exported from internal/scope.
const (
	ScopeFlagOrg  = scope.ScopeFlagOrg
	ScopeFlagRepo = scope.ScopeFlagRepo
)

// ScopeFor re-exports scope.ScopeFor. See that function for full docs.
func ScopeFor(name, topology, owner, repoFullName string) (flag, target string) {
	return scope.ScopeFor(name, topology, owner, repoFullName)
}

// IsSharedName re-exports scope.IsSharedName.
func IsSharedName(name string) bool { return scope.IsSharedName(name) }

// IsIdentityName re-exports scope.IsIdentityName.
func IsIdentityName(name string) bool { return scope.IsIdentityName(name) }
