package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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
			if name == project.ProjectVarName {
				return "PVT_test123", nil
			}
			return "", errors.New("not found")
		},
		SetRepoVariable:    func(o, r, n, v string) error { return nil },
		DeleteRepoVariable: func(o, r, n string) error { return nil },
		ReadAIVersion:      func(root string) (string, error) { return "v2.0.10", nil },
	}
}

func TestProjectInfoCmd_OutputsTopology(t *testing.T) {
	deps := fakeProjectDeps("owner", "myrepo")

	var buf bytes.Buffer
	if err := project.PrintInfo(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Single") {
		t.Errorf("expected Single topology in output, got:\n%s", out)
	}
	if !strings.Contains(out, "PVT_test123") {
		t.Errorf("expected project ID in output, got:\n%s", out)
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

	for _, want := range []string{"info", "check"} {
		if !subs[want] {
			t.Errorf("subcommand %q not registered under project", want)
		}
	}
}
