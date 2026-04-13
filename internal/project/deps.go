package project

import "github.com/eddiecarpenter/gh-agentic/internal/mount"

// Deps holds injectable dependencies for project subcommands.
// Tests supply fakes; production fills in real defaults.
type Deps struct {
	// RepoFullName is "owner/repo" for the current repository.
	RepoFullName string
	// Owner is the GitHub owner login.
	Owner string
	// RepoName is the repository name without owner.
	RepoName string
	// Root is the local repository root path.
	Root string

	FetchLinkedRepos    FetchLinkedReposFunc
	FetchProjectsForRepo FetchProjectsForRepoFunc
	GetRepoVariable     GetRepoVariableFunc
	SetRepoVariable     SetRepoVariableFunc
	DeleteRepoVariable  DeleteRepoVariableFunc
	ReadAIVersion       func(root string) (string, error)
}

// DefaultDeps returns production dependencies for the given repo context.
func DefaultDeps(owner, repoName, root string) Deps {
	return Deps{
		RepoFullName:        owner + "/" + repoName,
		Owner:               owner,
		RepoName:            repoName,
		Root:                root,
		FetchLinkedRepos:    DefaultFetchLinkedRepos,
		FetchProjectsForRepo: DefaultFetchProjectsForRepo,
		GetRepoVariable:     DefaultGetRepoVariable,
		SetRepoVariable:     DefaultSetRepoVariable,
		DeleteRepoVariable:  DefaultDeleteRepoVariable,
		ReadAIVersion:       mount.ReadAIVersionFromGit,
	}
}
