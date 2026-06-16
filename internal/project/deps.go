package project

import (
	"github.com/charmbracelet/huh"
	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

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

	FetchLinkedRepos     FetchLinkedReposFunc
	FetchProjectsForRepo FetchProjectsForRepoFunc
	GetRepoVariable      GetRepoVariableFunc
	SetRepoVariable      SetRepoVariableFunc
	DeleteRepoVariable   DeleteRepoVariableFunc
	ReadAIVersion        func(root string) (string, error)

	FetchOwnerAndRepoIDs     FetchOwnerAndRepoIDsFunc
	CreateProject            CreateProjectFunc
	LinkRepoToProject        LinkRepoToProjectFunc
	Confirm                  ConfirmFunc
	DetectOwnerType          auth.DetectOwnerTypeFunc
	Clone                    mount.CloneFunc
	FetchReleases            mount.FetchReleasesFunc
	UpdateProject            UpdateProjectFunc
	FetchProjectFields       FetchProjectFieldsFunc
	UpdateStatusFieldOptions UpdateStatusFieldOptionsFunc
	CreateProjectField       CreateProjectFieldFunc
	FetchProjectNumber       FetchProjectNumberFunc
	CreateProjectView        CreateProjectViewFunc
	FetchProjectViews        FetchProjectViewsFunc
	FetchProjectsForOwner    FetchProjectsForOwnerFunc
	FetchProjectTitle        FetchProjectTitleFunc
	FetchProjectOwner        FetchProjectOwnerFunc
	FetchOrphanIssues        FetchOrphanIssuesFunc
	AddIssueToProject        AddIssueToProjectFunc
	Run                      auth.RunCommandFunc
}

// DefaultDeps returns production dependencies for the given repo context.
func DefaultDeps(owner, repoName, root string) Deps {
	return Deps{
		RepoFullName:             owner + "/" + repoName,
		Owner:                    owner,
		RepoName:                 repoName,
		Root:                     root,
		FetchLinkedRepos:         DefaultFetchLinkedRepos,
		FetchProjectsForRepo:     DefaultFetchProjectsForRepo,
		GetRepoVariable:          DefaultGetRepoVariable,
		SetRepoVariable:          DefaultSetRepoVariable,
		DeleteRepoVariable:       DefaultDeleteRepoVariable,
		ReadAIVersion:            mount.ReadAIVersionFromGit,
		FetchOwnerAndRepoIDs:     DefaultFetchOwnerAndRepoIDs,
		CreateProject:            DefaultCreateProject,
		LinkRepoToProject:        DefaultLinkRepoToProject,
		Confirm:                  defaultConfirm,
		DetectOwnerType:          auth.DefaultDetectOwnerType,
		Clone:                    mount.DefaultClone,
		FetchReleases:            mount.DefaultFetchReleases,
		UpdateProject:            DefaultUpdateProject,
		FetchProjectFields:       DefaultFetchProjectFields,
		UpdateStatusFieldOptions: DefaultUpdateStatusFieldOptions,
		CreateProjectField:       DefaultCreateProjectField,
		FetchProjectNumber:       DefaultFetchProjectNumber,
		CreateProjectView:        DefaultCreateProjectView,
		FetchProjectViews:        DefaultFetchProjectViews,
		FetchProjectsForOwner:    DefaultFetchProjectsForOwner,
		FetchProjectTitle:        DefaultFetchProjectTitle,
		FetchProjectOwner:        DefaultFetchProjectOwner,
		FetchOrphanIssues:        DefaultFetchOrphanIssues,
		AddIssueToProject:        DefaultAddIssueToProject,
		Run:                      auth.DefaultRunCommand,
	}
}

// defaultConfirm prompts the user via huh for a yes/no confirmation.
func defaultConfirm(prompt string) (bool, error) {
	var confirmed bool
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title(prompt).Value(&confirmed),
		),
	).Run()
	if err != nil {
		return false, err
	}
	return confirmed, nil
}
