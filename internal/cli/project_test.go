package cli

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// fakeProjectDeps builds a project.Deps with injectable fakes for CLI tests.
func fakeProjectDeps(owner, repo string) project.Deps {
	return project.Deps{
		RepoFullName: owner + "/" + repo,
		Owner:        owner,
		RepoName:     repo,
		Root:         "/fake/root",
		FetchLinkedRepos: func(projectID string) ([]project.LinkedRepo, error) {
			return []project.LinkedRepo{
				{Name: repo, NameWithOwner: owner + "/" + repo},
			}, nil
		},
		FetchProjectsForRepo: func(o, r string) ([]project.ProjectInfo, error) {
			return nil, nil
		},
		GetRepoVariable: func(o, r, name string) (string, error) {
			switch name {
			case project.ProjectVarName:
				return "PVT_test123", nil
			case project.FrameworkVersionVarName:
				return "v2.0.10", nil
			case project.TopologyVarName:
				return "single", nil
			}
			return "", errors.New("not found")
		},
		SetRepoVariable:    func(o, r, n, v string) error { return nil },
		DeleteRepoVariable: func(o, r, n string) error { return nil },
		ReadAIVersion:      func(root string) (string, error) { return "v2.0.10", nil },
		FetchOwnerAndRepoIDs: func(owner, repo string) (string, string, error) {
			return "owner-node-id", "repo-node-id", nil
		},
		CreateProject:     func(ownerID, title string) (string, error) { return "PVT_created123", nil },
		LinkRepoToProject: func(projectID, repoID string) error { return nil },
		Confirm:           func(prompt string) (bool, error) { return true, nil },
		DetectOwnerType:   func(owner string) (string, error) { return "Organization", nil },
		Clone:             func(repoURL, tag, destDir string) error { return nil },
		FetchReleases: func(repo string) ([]mount.Release, error) {
			return []mount.Release{{TagName: "v2.0.10"}}, nil
		},
		UpdateProject: func(projectID, shortDescription, readme string) error { return nil },
		FetchProjectFields: func(projectID string) ([]project.ProjectField, error) {
			return []project.ProjectField{{ID: "field-id", Name: "Status", DataType: "SINGLE_SELECT"}}, nil
		},
		UpdateStatusFieldOptions: func(fieldID string, options []project.StatusOption) error { return nil },
		FetchProjectNumber: func(projectID string) (int, error) { return 1, nil },
		CreateProjectView: func(owner, ownerType string, projectNumber int, name, layout, filter string) error {
			return nil
		},
		FetchProjectViews: func(projectID string) ([]project.ProjectView, error) {
			return []project.ProjectView{{Name: "Requirements"}, {Name: "Requirements Kanban"}, {Name: "Features Kanban"}}, nil
		},
		FetchProjectsForOwner: func(owner, ownerType string) ([]project.ProjectInfo, error) {
			return []project.ProjectInfo{{ID: "PVT_test123", Title: "Test Project"}}, nil
		},
		FetchProjectTitle: func(projectID string) (string, error) {
			if projectID == "PVT_test123" {
				return "Test Project", nil
			}
			return "", nil
		},
	}
}

func TestProjectCheckCmd_PassesForHealthyRepo(t *testing.T) {
	deps := fakeProjectDeps("owner", "myrepo")
	report := project.RunChecks(deps)

	if report.FailCount() != 0 {
		t.Errorf("expected 0 failures, got %d", report.FailCount())
	}
}

func TestProjectCheckCmd_FailsWhenNoProjectID(t *testing.T) {
	deps := fakeProjectDeps("owner", "myrepo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}

	report := project.RunChecks(deps)
	if report.FailCount() == 0 {
		t.Error("expected failures when project ID is missing")
	}
}

func TestProjectCmd_Registered(t *testing.T) {
	root := newRootCmd("dev", "")
	found := false
	for _, c := range root.Commands() {
		if c.Use == "project" {
			found = true
			break
		}
	}
	if !found {
		t.Error("project command not registered on root")
	}
}

func TestProjectCmd_SubcommandsRegistered(t *testing.T) {
	root := newRootCmd("dev", "")
	var projectCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Use == "project" {
			projectCmd = c
			break
		}
	}
	if projectCmd == nil {
		t.Fatal("project command not found")
	}

	subs := map[string]bool{}
	for _, c := range projectCmd.Commands() {
		subs[c.Use] = true
	}

	for _, want := range []string{"check", "create [title]", "join [project-name]", "unlink", "repair", "switch", "init"} {
		if !subs[want] {
			t.Errorf("subcommand %q not registered under project", want)
		}
	}
}
