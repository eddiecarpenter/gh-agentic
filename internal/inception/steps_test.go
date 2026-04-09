package inception

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// --------------------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------------------

// fakeRunOK returns a RunCommandFunc that always succeeds with the given output.
func fakeRunOK(out string) bootstrap.RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		return out, nil
	}
}

// fakeRunFail returns a RunCommandFunc that always fails with the given error.
func fakeRunFail(msg string) bootstrap.RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		return msg, errors.New(msg)
	}
}

// --------------------------------------------------------------------------------------
// Step 1 — CreateRepo
// --------------------------------------------------------------------------------------

func TestCreateRepo_Success_PopulatesState(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{
		RepoType: "domain",
		RepoName: "charging",
		Owner:    "acme-org",
	}
	state := &StepState{}
	env := &EnvContext{AgenticRepoRoot: dir, Owner: "acme-org", TemplateRepo: bootstrap.DefaultTemplateRepo}

	run := fakeRunOK("")

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, env, run)
	if err != nil {
		t.Fatalf("CreateRepo() unexpected error: %v", err)
	}

	if state.RepoName != "charging-domain" {
		t.Errorf("state.RepoName = %q, want %q", state.RepoName, "charging-domain")
	}
	if state.RepoURL != "https://github.com/acme-org/charging-domain" {
		t.Errorf("state.RepoURL = %q, want %q", state.RepoURL, "https://github.com/acme-org/charging-domain")
	}
	expectedClone := filepath.Join(dir, "domains", "charging-domain")
	if state.ClonePath != expectedClone {
		t.Errorf("state.ClonePath = %q, want %q", state.ClonePath, expectedClone)
	}
}

func TestCreateRepo_GhCreateFails_ReturnsError(t *testing.T) {
	cfg := &InceptionConfig{RepoType: "domain", RepoName: "charging", Owner: "acme"}
	state := &StepState{}
	env := &EnvContext{AgenticRepoRoot: t.TempDir(), TemplateRepo: bootstrap.DefaultTemplateRepo}

	run := fakeRunFail("already exists")

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, env, run)
	if err == nil {
		t.Fatal("CreateRepo() expected error when gh repo create fails, got nil")
	}
	if !strings.Contains(err.Error(), "gh repo create") {
		t.Errorf("error should mention 'gh repo create', got: %v", err)
	}
}

func TestCreateRepo_CloneFails_ReturnsError(t *testing.T) {
	cfg := &InceptionConfig{RepoType: "tool", RepoName: "bench", Owner: "acme"}
	state := &StepState{}
	env := &EnvContext{AgenticRepoRoot: t.TempDir(), TemplateRepo: bootstrap.DefaultTemplateRepo}

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "https://github.com/acme/bench-tool", nil
		}
		return "fatal: repository not found", errors.New("exit status 128")
	}

	var buf bytes.Buffer
	err := CreateRepo(&buf, cfg, state, env, run)
	if err == nil {
		t.Fatal("CreateRepo() expected error when git clone fails, got nil")
	}
	if !strings.Contains(err.Error(), "git clone") {
		t.Errorf("error should mention 'git clone', got: %v", err)
	}
}

func TestCreateRepo_OtherType_UsesOthersDir(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{RepoType: "other", RepoName: "my-service", Owner: "alice"}
	state := &StepState{}
	env := &EnvContext{AgenticRepoRoot: dir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	_ = CreateRepo(&buf, cfg, state, env, fakeRunOK(""))

	expectedClone := filepath.Join(dir, "others", "my-service")
	if state.ClonePath != expectedClone {
		t.Errorf("state.ClonePath = %q, want %q", state.ClonePath, expectedClone)
	}
}

// --------------------------------------------------------------------------------------
// Step 2 — ConfigureLabels
// --------------------------------------------------------------------------------------

func TestConfigureLabels_AllLabelsCreated_ReturnsNil(t *testing.T) {
	cfg := &InceptionConfig{Owner: "acme"}
	state := &StepState{RepoName: "charging-domain"}

	var created []string
	run := func(name string, args ...string) (string, error) {
		if name == "gh" && len(args) > 2 && args[0] == "label" && args[1] == "create" {
			created = append(created, args[2])
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ConfigureLabels(&buf, cfg, state, run); err != nil {
		t.Fatalf("ConfigureLabels() unexpected error: %v", err)
	}
	if len(created) != len(standardLabels) {
		t.Errorf("expected %d labels created, got %d: %v", len(standardLabels), len(created), created)
	}
}

func TestConfigureLabels_OneLabelFails_StillReturnsNil(t *testing.T) {
	cfg := &InceptionConfig{Owner: "acme"}
	state := &StepState{RepoName: "charging-domain"}

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "already exists", errors.New("already exists")
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ConfigureLabels(&buf, cfg, state, run); err != nil {
		t.Errorf("ConfigureLabels() should not return error on label failure (best-effort), got: %v", err)
	}
}

// --------------------------------------------------------------------------------------
// Step 3 — ScaffoldStacks
// --------------------------------------------------------------------------------------

func TestScaffoldStacks_OtherStack_SkipsWithWarning(t *testing.T) {
	cfg := &InceptionConfig{Stacks: []string{"Other"}}
	state := &StepState{ClonePath: t.TempDir()}
	env := &EnvContext{AgenticRepoRoot: t.TempDir(), TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, env, fakeRunFail("should not be called")); err != nil {
		t.Fatalf("ScaffoldStacks() expected nil for 'Other' stack, got: %v", err)
	}
	if !strings.Contains(buf.String(), "skipping scaffold") {
		t.Errorf("expected skip message in output, got: %s", buf.String())
	}
}

func TestScaffoldStacks_MissingStandardsFile_ReturnsError(t *testing.T) {
	cfg := &InceptionConfig{Stacks: []string{"Go"}}
	state := &StepState{ClonePath: t.TempDir()}
	env := &EnvContext{AgenticRepoRoot: t.TempDir(), TemplateRepo: bootstrap.DefaultTemplateRepo} // No base/standards/go.md

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, env, fakeRunOK("")); err == nil {
		t.Error("ScaffoldStacks() expected error when standards file missing, got nil")
	}
}

func TestScaffoldStacks_ExecutesCommandsFromFile(t *testing.T) {
	agenticDir := t.TempDir()
	stdDir := filepath.Join(agenticDir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "## Project Initialisation\n\n```bash\necho hello\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{Stacks: []string{"Go"}}
	state := &StepState{ClonePath: t.TempDir()}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	var executed []string
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			executed = append(executed, strings.Join(args, " "))
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, env, run); err != nil {
		t.Fatalf("ScaffoldStacks() unexpected error: %v", err)
	}
	if len(executed) == 0 {
		t.Error("expected at least one bash command to be executed")
	}
}

func TestScaffoldStacks_MultipleStacks_ExecutesAll(t *testing.T) {
	agenticDir := t.TempDir()
	stdDir := filepath.Join(agenticDir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}

	goContent := "## Project Initialisation\n\n```bash\necho go-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}
	rustContent := "## Project Initialisation\n\n```bash\necho rust-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "rust.md"), []byte(rustContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{Stacks: []string{"Go", "Rust"}}
	state := &StepState{ClonePath: t.TempDir()}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	var executed []string
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			executed = append(executed, strings.Join(args, " "))
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, env, run); err != nil {
		t.Fatalf("ScaffoldStacks() unexpected error: %v", err)
	}
	if len(executed) != 2 {
		t.Errorf("expected 2 commands executed (one per stack), got %d", len(executed))
	}
}

func TestScaffoldStacks_SkipsTrackWithNoInitSection(t *testing.T) {
	agenticDir := t.TempDir()
	stdDir := filepath.Join(agenticDir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}

	goContent := "## Project Initialisation\n\n```bash\necho go-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}
	pyContent := "## Build Verification\n\nSome build info.\n"
	if err := os.WriteFile(filepath.Join(stdDir, "python.md"), []byte(pyContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{Stacks: []string{"Go", "Python"}}
	state := &StepState{ClonePath: t.TempDir()}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	var executed []string
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			executed = append(executed, strings.Join(args, " "))
		}
		return "", nil
	}

	var buf bytes.Buffer
	if err := ScaffoldStacks(&buf, cfg, state, env, run); err != nil {
		t.Fatalf("ScaffoldStacks() unexpected error: %v", err)
	}
	if len(executed) != 1 {
		t.Errorf("expected 1 command executed (Python skipped), got %d", len(executed))
	}
	if !strings.Contains(buf.String(), "skipping") {
		t.Errorf("expected skip message in output, got: %s", buf.String())
	}
}

func TestScaffoldStacks_HaltsOnFailure_ReportsTrack(t *testing.T) {
	agenticDir := t.TempDir()
	stdDir := filepath.Join(agenticDir, "base", "standards")
	if err := os.MkdirAll(stdDir, 0755); err != nil {
		t.Fatal(err)
	}

	goContent := "## Project Initialisation\n\n```bash\necho go-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "go.md"), []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}
	rustContent := "## Project Initialisation\n\n```bash\necho rust-init\n```\n\n## Other\n"
	if err := os.WriteFile(filepath.Join(stdDir, "rust.md"), []byte(rustContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{Stacks: []string{"Go", "Rust"}}
	state := &StepState{ClonePath: t.TempDir()}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	callCount := 0
	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			callCount++
			if callCount == 2 {
				return "command failed", errors.New("exit status 1")
			}
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := ScaffoldStacks(&buf, cfg, state, env, run)
	if err == nil {
		t.Fatal("expected error when command fails, got nil")
	}
	if !strings.Contains(err.Error(), "[Rust]") {
		t.Errorf("error should mention the failing track 'Rust', got: %v", err)
	}
}

// --------------------------------------------------------------------------------------
// Step 4 — PopulateRepo
// --------------------------------------------------------------------------------------

func TestPopulateRepo_WritesThreeFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{
		RepoType:    "domain",
		RepoName:    "charging",
		Description: "OCS charging engine",
		Stacks:     []string{"Go"},
		Owner:       "acme-org",
	}
	state := &StepState{
		RepoName:  "charging-domain",
		ClonePath: dir,
		RepoURL:   "https://github.com/acme-org/charging-domain",
	}
	env := &EnvContext{AgenticRepoRoot: t.TempDir(), Owner: "acme-org", TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	for _, f := range []string{"CLAUDE.md", "AGENTS.local.md", "README.md"} {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("expected %s to exist, got: %v", f, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", f)
		}
	}
}

func TestPopulateRepo_CLAUDEMDReferencesAGENTSMD(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: dir}
	env := &EnvContext{AgenticRepoRoot: t.TempDir(), TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	_ = PopulateRepo(&buf, cfg, state, env, fakeRunOK(""))

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if !strings.Contains(string(data), "AGENTS.md") {
		t.Errorf("CLAUDE.md should reference AGENTS.md, got:\n%s", string(data))
	}
}

func TestPopulateRepo_PushFails_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: dir}
	env := &EnvContext{AgenticRepoRoot: t.TempDir(), TemplateRepo: bootstrap.DefaultTemplateRepo}

	run := func(name string, args ...string) (string, error) {
		if name == "bash" {
			script := strings.Join(args, " ")
			if strings.Contains(script, "'git'") && strings.Contains(script, "'push'") {
				return "error: failed to push", errors.New("exit status 1")
			}
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := PopulateRepo(&buf, cfg, state, env, run)
	if err == nil {
		t.Fatal("PopulateRepo() expected error when git push fails, got nil")
	}
	if !strings.Contains(err.Error(), "git push") {
		t.Errorf("error should mention 'git push', got: %v", err)
	}
}

func TestPopulateRepo_CopiesBase(t *testing.T) {
	cloneDir := t.TempDir()
	agenticDir := t.TempDir()

	// Create a base/ directory with a file.
	baseDir := filepath.Join(agenticDir, "base", "standards")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "go.md"), []byte("# Go standards"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: cloneDir}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Verify base/ was copied.
	data, err := os.ReadFile(filepath.Join(cloneDir, "base", "standards", "go.md"))
	if err != nil {
		t.Fatalf("expected base/standards/go.md to be copied: %v", err)
	}
	if string(data) != "# Go standards" {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestPopulateRepo_CopiesGooseRecipes(t *testing.T) {
	cloneDir := t.TempDir()
	agenticDir := t.TempDir()

	// Create .goose/recipes/ with YAML files.
	recipesDir := filepath.Join(agenticDir, ".goose", "recipes")
	if err := os.MkdirAll(recipesDir, 0755); err != nil {
		t.Fatal(err)
	}
	recipeFiles := []string{
		"dev-session.yaml", "feature-design.yaml", "feature-scoping.yaml",
		"foreground-recovery.yaml", "issue-session.yaml", "pr-review-session.yaml",
		"requirements-session.yaml",
	}
	for _, name := range recipeFiles {
		if err := os.WriteFile(filepath.Join(recipesDir, name), []byte("recipe: "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: cloneDir}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Verify all 7 recipe YAMLs were copied.
	for _, name := range recipeFiles {
		path := filepath.Join(cloneDir, ".goose", "recipes", name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected recipe %s to be copied", name)
		}
	}
}

func TestPopulateRepo_CopiesWorkflows_ExcludesCIYml(t *testing.T) {
	cloneDir := t.TempDir()
	agenticDir := t.TempDir()

	// Create .github/workflows/ with workflow files including ci.yml.
	workflowsDir := filepath.Join(agenticDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatal(err)
	}
	workflowFiles := []string{
		"ci.yml", "dev-session.yml", "feature-complete.yml",
		"feature-design.yml", "issue-session.yml", "pr-review-session.yml",
		"sync-status-to-label.yml",
	}
	for _, name := range workflowFiles {
		if err := os.WriteFile(filepath.Join(workflowsDir, name), []byte("workflow: "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: cloneDir}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo, OwnerType: bootstrap.OwnerTypeOrg}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Verify pipeline workflows were copied (6 files including sync-status-to-label.yml, NOT ci.yml).
	expectedFiles := []string{"dev-session.yml", "feature-complete.yml", "feature-design.yml", "issue-session.yml", "pr-review-session.yml", "sync-status-to-label.yml"}
	for _, name := range expectedFiles {
		path := filepath.Join(cloneDir, ".github", "workflows", name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected workflow %s to be copied for org owner", name)
		}
	}

	// Verify ci.yml was NOT copied.
	ciPath := filepath.Join(cloneDir, ".github", "workflows", "ci.yml")
	if _, err := os.Stat(ciPath); err == nil {
		t.Error("ci.yml should NOT be copied to the domain repo")
	}
}

func TestPopulateRepo_UserOwner_SkipsSyncStatusToLabel(t *testing.T) {
	cloneDir := t.TempDir()
	agenticDir := t.TempDir()

	// Create .github/workflows/ with workflow files including sync-status-to-label.yml.
	workflowsDir := filepath.Join(agenticDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatal(err)
	}
	workflowFiles := []string{
		"ci.yml", "dev-session.yml", "feature-complete.yml",
		"feature-design.yml", "sync-status-to-label.yml",
	}
	for _, name := range workflowFiles {
		if err := os.WriteFile(filepath.Join(workflowsDir, name), []byte("workflow: "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &InceptionConfig{Owner: "alice", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: cloneDir}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo, OwnerType: bootstrap.OwnerTypeUser}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Verify pipeline workflows were copied (3 files, NOT ci.yml or sync-status-to-label.yml).
	expectedFiles := []string{"dev-session.yml", "feature-complete.yml", "feature-design.yml"}
	for _, name := range expectedFiles {
		path := filepath.Join(cloneDir, ".github", "workflows", name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected workflow %s to be copied", name)
		}
	}

	// Verify ci.yml was NOT copied.
	ciPath := filepath.Join(cloneDir, ".github", "workflows", "ci.yml")
	if _, err := os.Stat(ciPath); err == nil {
		t.Error("ci.yml should NOT be copied to the domain repo")
	}

	// Verify sync-status-to-label.yml was NOT copied for user owner.
	syncPath := filepath.Join(cloneDir, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); err == nil {
		t.Error("sync-status-to-label.yml should NOT be copied for personal (user) repos")
	}
}

func TestPopulateRepo_OrgOwner_IncludesSyncStatusToLabel(t *testing.T) {
	cloneDir := t.TempDir()
	agenticDir := t.TempDir()

	// Create .github/workflows/ with sync-status-to-label.yml.
	workflowsDir := filepath.Join(agenticDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatal(err)
	}
	workflowFiles := []string{
		"dev-session.yml", "sync-status-to-label.yml",
	}
	for _, name := range workflowFiles {
		if err := os.WriteFile(filepath.Join(workflowsDir, name), []byte("workflow: "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: cloneDir}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo, OwnerType: bootstrap.OwnerTypeOrg}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	// Verify sync-status-to-label.yml WAS copied for org owner.
	syncPath := filepath.Join(cloneDir, ".github", "workflows", "sync-status-to-label.yml")
	if _, err := os.Stat(syncPath); os.IsNotExist(err) {
		t.Error("sync-status-to-label.yml should be copied for org repos")
	}
}

func TestPopulateRepo_UpdatesGitignore(t *testing.T) {
	cloneDir := t.TempDir()
	agenticDir := t.TempDir()

	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: cloneDir}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, ".gitignore"))
	if err != nil {
		t.Fatalf("expected .gitignore to be created: %v", err)
	}
	content := string(data)
	for _, entry := range []string{".goose/config/", ".goose/data/", ".goose/state/"} {
		if !strings.Contains(content, entry) {
			t.Errorf(".gitignore should contain %q, got:\n%s", entry, content)
		}
	}
}

func TestPopulateRepo_GitignoreIdempotent(t *testing.T) {
	cloneDir := t.TempDir()
	agenticDir := t.TempDir()

	// Pre-create a .gitignore with the entries.
	existing := ".goose/config/\n.goose/data/\n.goose/state/\n"
	if err := os.WriteFile(filepath.Join(cloneDir, ".gitignore"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{Owner: "acme-org", RepoType: "domain", RepoName: "charging"}
	state := &StepState{RepoName: "charging-domain", ClonePath: cloneDir}
	env := &EnvContext{AgenticRepoRoot: agenticDir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	if err := PopulateRepo(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("PopulateRepo() unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(cloneDir, ".gitignore"))
	content := string(data)
	// Should not duplicate entries.
	if strings.Count(content, ".goose/config/") != 1 {
		t.Errorf("expected .goose/config/ once, got:\n%s", content)
	}
}

// --------------------------------------------------------------------------------------
// Step 5 — RegisterInREPOS
// --------------------------------------------------------------------------------------

func TestRegisterInREPOS_AppendsEntry(t *testing.T) {
	dir := t.TempDir()
	reposPath := filepath.Join(dir, "REPOS.md")
	if err := os.WriteFile(reposPath, []byte("# REPOS.md\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &InceptionConfig{
		RepoType:    "domain",
		RepoName:    "charging",
		Description: "OCS charging engine",
		Stacks:     []string{"Go"},
		Owner:       "acme-org",
	}
	state := &StepState{RepoName: "charging-domain"}
	env := &EnvContext{AgenticRepoRoot: dir, TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	if err := RegisterInREPOS(&buf, cfg, state, env, fakeRunOK("")); err != nil {
		t.Fatalf("RegisterInREPOS() unexpected error: %v", err)
	}

	data, err := os.ReadFile(reposPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"charging-domain",
		"acme-org",
		"Go",
		"domain",
		"active",
		"OCS charging engine",
	}
	for _, want := range checks {
		if !strings.Contains(content, want) {
			t.Errorf("REPOS.md should contain %q, got:\n%s", want, content)
		}
	}
}

func TestRegisterInREPOS_ReadFails_ReturnsError(t *testing.T) {
	cfg := &InceptionConfig{Owner: "acme"}
	state := &StepState{RepoName: "charging-domain"}
	env := &EnvContext{AgenticRepoRoot: "/nonexistent", TemplateRepo: bootstrap.DefaultTemplateRepo}

	var buf bytes.Buffer
	err := RegisterInREPOS(&buf, cfg, state, env, fakeRunOK(""))
	if err == nil {
		t.Fatal("RegisterInREPOS() expected error when REPOS.md missing, got nil")
	}
}

// --------------------------------------------------------------------------------------
// Step 6 — PrintSummary
// --------------------------------------------------------------------------------------

func TestPrintSummary_OutputContainsAllFields(t *testing.T) {
	cfg := &InceptionConfig{RepoType: "domain"}
	state := &StepState{
		RepoURL:   "https://github.com/acme-org/charging-domain",
		ClonePath: "/home/user/domains/charging-domain",
	}

	var buf bytes.Buffer
	PrintSummary(&buf, cfg, state)

	out := buf.String()
	if !strings.Contains(out, "Inception complete") {
		t.Errorf("expected 'Inception complete' in output, got: %s", out)
	}
	if !strings.Contains(out, state.RepoURL) {
		t.Errorf("expected repo URL in output, got: %s", out)
	}
	if !strings.Contains(out, state.ClonePath) {
		t.Errorf("expected clone path in output, got: %s", out)
	}
	if !strings.Contains(out, "domain") {
		t.Errorf("expected repo type in output, got: %s", out)
	}
}

// --------------------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------------------

func TestExtractInitCommands_FindsSection(t *testing.T) {
	content := "## Project Initialisation\n\n```bash\necho hello\n```\n\n## Other\n"
	cmds, err := extractInitCommands(content)
	if err != nil {
		t.Fatalf("extractInitCommands() error: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command block, got %d", len(cmds))
	}
	if !strings.Contains(cmds[0], "echo hello") {
		t.Errorf("expected 'echo hello' in command, got: %s", cmds[0])
	}
}

func TestExtractInitCommands_SectionMissing_ReturnsError(t *testing.T) {
	_, err := extractInitCommands("# No relevant section")
	if err == nil {
		t.Error("expected error when section is missing, got nil")
	}
}

func TestShellQuote_EscapesSingleQuotes(t *testing.T) {
	got := shellQuote("it's")
	if !strings.Contains(got, `'\''`) {
		t.Errorf("shellQuote should escape single quotes, got: %s", got)
	}
}
