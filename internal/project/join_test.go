package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- JoinDomain ---

func TestJoinDomain_RegistersRepo_NoMount(t *testing.T) {
	tmp := t.TempDir()
	deps := testDeps("cp-owner", "cp")
	deps.Root = tmp
	var setOwner, setRepo, setVal string
	deps.SetRepoVariable = func(o, r, n, v string) error {
		if n == ProjectVarName {
			setOwner, setRepo, setVal = o, r, v
		}
		return nil
	}
	deps.FetchOwnerAndRepoIDs = func(o, r string) (string, string, error) {
		return "ownerID", "repoID-" + r, nil
	}
	var linkedRepoID string
	deps.LinkRepoToProject = func(projectID, repoID string) error {
		linkedRepoID = repoID
		return nil
	}

	var buf bytes.Buffer
	if err := JoinDomain(&buf, deps, "PVT_cp", "cp-owner", "billing-svc", "billing", "Billing domain", "Bill runs"); err != nil {
		t.Fatalf("JoinDomain: %v", err)
	}

	fed, err := ReadFederation(tmp)
	if err != nil {
		t.Fatalf("ReadFederation after join: %v", err)
	}
	if !fed.HasDomain("billing") {
		t.Error("expected the billing domain to be created")
	}
	all := fed.AllRepos()
	if len(all) != 1 || all[0].Name != "cp-owner/billing-svc" {
		t.Errorf("expected cp-owner/billing-svc registered, got %+v", all)
	}
	if setOwner != "cp-owner" || setRepo != "billing-svc" || setVal != "PVT_cp" {
		t.Errorf("expected AGENTIC_PROJECT_ID=PVT_cp on cp-owner/billing-svc, got %s/%s=%s", setOwner, setRepo, setVal)
	}
	if linkedRepoID != "repoID-billing-svc" {
		t.Errorf("expected the target repo linked to the project, got %q", linkedRepoID)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents")); !os.IsNotExist(err) {
		t.Error("JoinDomain must not mount the framework into the control-plane working dir")
	}
}

func TestJoinDomain_RejectsDuplicateRepo(t *testing.T) {
	tmp := t.TempDir()
	deps := testDeps("cp-owner", "cp")
	deps.Root = tmp
	deps.SetRepoVariable = func(o, r, n, v string) error { return nil }
	deps.FetchOwnerAndRepoIDs = func(o, r string) (string, string, error) { return "o", "r", nil }
	deps.LinkRepoToProject = func(p, r string) error { return nil }

	var buf bytes.Buffer
	if err := JoinDomain(&buf, deps, "PVT_cp", "cp-owner", "svc", "charging", "C", "p"); err != nil {
		t.Fatalf("first JoinDomain: %v", err)
	}
	if err := JoinDomain(&buf, deps, "PVT_cp", "cp-owner", "svc", "billing", "B", "p"); err == nil {
		t.Fatal("expected a duplicate-repo error on re-registering the same repo")
	}
}

// --- Unlink ---

func TestUnlink_Clear_NothingToUnlink(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "", errors.New("not set")
	}

	var deleteCalled bool
	deps.DeleteRepoVariable = func(o, r, n string) error {
		deleteCalled = true
		return nil
	}

	var buf bytes.Buffer
	if err := Unlink(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalled {
		t.Error("expected DeleteRepoVariable NOT to be called when nothing to unlink")
	}
	if !strings.Contains(buf.String(), "nothing to unlink") {
		t.Errorf("expected 'nothing to unlink' in output, got:\n%s", buf.String())
	}
}

func TestUnlink_WarnConfirm_Confirmed(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "other", NameWithOwner: "owner/other"}}, nil
	}
	deps.Confirm = func(prompt string) (bool, error) { return true, nil }

	var deleteCalled bool
	deps.DeleteRepoVariable = func(o, r, n string) error {
		deleteCalled = true
		return nil
	}

	var buf bytes.Buffer
	if err := Unlink(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Error("expected DeleteRepoVariable to be called")
	}
}

func TestUnlink_WarnConfirm_Denied(t *testing.T) {
	deps := testDeps("owner", "repo")
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "other", NameWithOwner: "owner/other"}}, nil
	}
	deps.Confirm = func(prompt string) (bool, error) { return false, nil }

	var buf bytes.Buffer
	err := Unlink(&buf, deps)
	if err == nil {
		t.Fatal("expected error when user denies")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected 'aborted' in error, got: %v", err)
	}
}

func TestUnlink_Blocked_SingleWithDocsContent(t *testing.T) {
	tmp := t.TempDir()
	docsDir := tmp + "/docs"
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docsDir+"/readme.md", []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := testDeps("owner", "repo")
	deps.Root = tmp
	deps.GetRepoVariable = func(o, r, n string) (string, error) {
		return "PVT_existing", nil
	}
	deps.FetchLinkedRepos = func(projectID string) ([]LinkedRepo, error) {
		return []LinkedRepo{{Name: "repo", NameWithOwner: "owner/repo"}}, nil
	}

	var buf bytes.Buffer
	err := Unlink(&buf, deps)
	if err == nil {
		t.Fatal("expected error when unlink is blocked")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected 'blocked' in error, got: %v", err)
	}
}
