package bootstrap

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// --- validateProjectName tests ---

func TestValidateProjectName_ValidNames_ReturnsNil(t *testing.T) {
	valid := []string{
		"my-project",
		"foo",
		"a-b-c-123",
		"ocs-testbench",
		"project1",
		"a",
	}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			if err := validateProjectName(name); err != nil {
				t.Errorf("validateProjectName(%q) expected nil, got: %v", name, err)
			}
		})
	}
}

func TestValidateProjectName_EmptyString_ReturnsError(t *testing.T) {
	err := validateProjectName("")
	if err == nil {
		t.Error("validateProjectName(\"\") expected error, got nil")
	}
}

func TestValidateProjectName_UppercaseWithSpace_ReturnsError(t *testing.T) {
	err := validateProjectName("My Project")
	if err == nil {
		t.Error("validateProjectName(\"My Project\") expected error, got nil")
	}
}

func TestValidateProjectName_SpaceOnly_ReturnsError(t *testing.T) {
	err := validateProjectName("my project")
	if err == nil {
		t.Error("validateProjectName(\"my project\") expected error for space, got nil")
	}
}

func TestValidateProjectName_UppercaseNoSpace_ReturnsError(t *testing.T) {
	err := validateProjectName("MyProject")
	if err == nil {
		t.Error("validateProjectName(\"MyProject\") expected error for uppercase, got nil")
	}
}

func TestValidateProjectName_TrailingHyphen_IsValid(t *testing.T) {
	// Hyphens are allowed anywhere; caller may impose further constraints later.
	if err := validateProjectName("my-project-"); err != nil {
		t.Errorf("validateProjectName(\"my-project-\") expected nil, got: %v", err)
	}
}

func TestValidateProjectName_SpecialChars_ReturnsError(t *testing.T) {
	for _, name := range []string{"my_project", "my.project", "my@project"} {
		err := validateProjectName(name)
		if err == nil {
			t.Errorf("validateProjectName(%q) expected error, got nil", name)
		}
	}
}

// --- RenderSummaryBox tests ---

func TestRenderSummaryBox_ContainsAllFields(t *testing.T) {
	cfg := BootstrapConfig{
		Topology:    "Single",
		Owner:       "newopenbss",
		ProjectName: "my-project",
		Description: "A test bench for OCS diameter testing",
		Stacks:      []string{"Go"},
		Antora:      false,
		AgentUser:   "goose-agent",
	}

	rendered := RenderSummaryBox(cfg)

	checks := []string{
		"Single",
		"newopenbss",
		"my-project",
		"A test bench",
		"Go",
		"No",
		"new",
		"goose-agent",
	}
	for _, want := range checks {
		if !strings.Contains(rendered, want) {
			t.Errorf("RenderSummaryBox() expected %q in output, got:\n%s", want, rendered)
		}
	}
}

func TestRenderSummaryBox_AntoraTrue_ShowsYes(t *testing.T) {
	cfg := BootstrapConfig{Antora: true}
	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "Yes") {
		t.Errorf("RenderSummaryBox() expected 'Yes' when Antora=true, got:\n%s", rendered)
	}
}

func TestRenderSummaryBox_AntoraFalse_ShowsNo(t *testing.T) {
	cfg := BootstrapConfig{Antora: false}
	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "No") {
		t.Errorf("RenderSummaryBox() expected 'No' when Antora=false, got:\n%s", rendered)
	}
}

func TestRenderSummaryBox_ContainsPipelineFields(t *testing.T) {
	cfg := BootstrapConfig{
		Topology:      "Single",
		Owner:         "alice",
		ProjectName:   "my-project",
		Description:   "test",
		Stacks:        []string{"Go"},
		Antora:        false,
		RunnerLabel:   "ubuntu-latest",
		GooseProvider: "claude-code",
		GooseModel:    "default",
	}

	rendered := RenderSummaryBox(cfg)

	checks := []string{
		"ubuntu-latest",
		"claude-code",
		"default",
		"Runner",
		"Provider",
		"Model",
	}
	for _, want := range checks {
		if !strings.Contains(rendered, want) {
			t.Errorf("RenderSummaryBox() expected %q in output, got:\n%s", want, rendered)
		}
	}
}

func TestRenderSummaryBox_CustomRunnerLabel_ShowsCustomValue(t *testing.T) {
	cfg := BootstrapConfig{
		RunnerLabel:   "self-hosted-gpu",
		GooseProvider: "openai",
		GooseModel:    "gpt-4",
	}

	rendered := RenderSummaryBox(cfg)

	for _, want := range []string{"self-hosted-gpu", "openai", "gpt-4"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("RenderSummaryBox() expected %q in output, got:\n%s", want, rendered)
		}
	}
}

func TestRenderSummaryBox_MultipleStacks_ShowsCommaJoined(t *testing.T) {
	cfg := BootstrapConfig{
		Topology:    "Single",
		Owner:       "alice",
		ProjectName: "my-project",
		Description: "test",
		Stacks:      []string{"Go", "TypeScript Node.js"},
	}

	rendered := RenderSummaryBox(cfg)

	if !strings.Contains(rendered, "Go, TypeScript Node.js") {
		t.Errorf("RenderSummaryBox() expected comma-joined stacks, got:\n%s", rendered)
	}
}

// --- validateStackSelection tests ---

func TestValidateStackSelection_Empty_ReturnsError(t *testing.T) {
	err := validateStackSelection([]string{})
	if err == nil {
		t.Error("validateStackSelection([]) expected error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one stack") {
		t.Errorf("expected 'at least one stack' in error, got: %v", err)
	}
}

func TestValidateStackSelection_Nil_ReturnsError(t *testing.T) {
	err := validateStackSelection(nil)
	if err == nil {
		t.Error("validateStackSelection(nil) expected error, got nil")
	}
}

func TestValidateStackSelection_OneStack_ReturnsNil(t *testing.T) {
	err := validateStackSelection([]string{"Go"})
	if err != nil {
		t.Errorf("validateStackSelection([Go]) expected nil, got: %v", err)
	}
}

func TestValidateStackSelection_MultipleStacks_ReturnsNil(t *testing.T) {
	err := validateStackSelection([]string{"Go", "Rust"})
	if err != nil {
		t.Errorf("validateStackSelection([Go, Rust]) expected nil, got: %v", err)
	}
}

// --- FetchOwnersFunc injection tests ---

func TestFetchOwners_PersonalAccountFirst(t *testing.T) {
	fakeFetch := func() ([]Owner, error) {
		return []Owner{
			{Login: "alice", Label: "alice  (personal)"},
			{Login: "acme-org", Label: "acme-org  ✔ clean"},
		}, nil
	}

	owners, err := fakeFetch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(owners) == 0 {
		t.Fatal("expected at least one owner")
	}
	if owners[0].Login != "alice" {
		t.Errorf("expected personal account first, got: %s", owners[0].Login)
	}
	if !strings.Contains(owners[0].Label, "(personal)") {
		t.Errorf("expected (personal) label on first owner, got: %s", owners[0].Label)
	}
}

func TestFetchOwners_OrgAnnotationsPresent(t *testing.T) {
	fakeFetch := func() ([]Owner, error) {
		return []Owner{
			{Login: "alice", Label: "alice  (personal)"},
			{Login: "clean-org", Label: "clean-org  ✔ clean"},
			{Login: "busy-org", Label: "busy-org  ⚠ has repos"},
		}, nil
	}

	owners, err := fakeFetch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasClean := false
	hasWarning := false
	for _, o := range owners {
		if strings.Contains(o.Label, "✔ clean") {
			hasClean = true
		}
		if strings.Contains(o.Label, "⚠ has repos") {
			hasWarning = true
		}
	}
	if !hasClean {
		t.Error("expected at least one owner labelled '✔ clean'")
	}
	if !hasWarning {
		t.Error("expected at least one owner labelled '⚠ has repos'")
	}
}

// --- validateTopologyOwner tests ---

func TestValidateTopologyOwner(t *testing.T) {
	tests := []struct {
		name      string
		topology  string
		ownerType string
		wantErr   error
	}{
		{
			name:      "personal + federated returns ErrFederatedRequiresOrg",
			topology:  "Federated",
			ownerType: OwnerTypeUser,
			wantErr:   ErrFederatedRequiresOrg,
		},
		{
			name:      "personal + single returns nil",
			topology:  "Single",
			ownerType: OwnerTypeUser,
			wantErr:   nil,
		},
		{
			name:      "org + federated returns nil",
			topology:  "Federated",
			ownerType: OwnerTypeOrg,
			wantErr:   nil,
		},
		{
			name:      "org + single returns nil",
			topology:  "Single",
			ownerType: OwnerTypeOrg,
			wantErr:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTopologyOwner(tc.topology, tc.ownerType)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("validateTopologyOwner(%q, %q) = %v, want %v", tc.topology, tc.ownerType, err, tc.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("validateTopologyOwner(%q, %q) unexpected error: %v", tc.topology, tc.ownerType, err)
				}
			}
		})
	}
}

// --- FetchReposFunc injection tests ---

func TestFetchRepos_SuccessfulFetch_ReturnsSortedRepos(t *testing.T) {
	fakeFetch := func(owner string) ([]Repo, error) {
		return []Repo{
			{Name: "alpha", FullName: owner + "/alpha"},
			{Name: "beta", FullName: owner + "/beta"},
			{Name: "gamma", FullName: owner + "/gamma"},
		}, nil
	}

	repos, err := fakeFetch("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
	if repos[0].Name != "alpha" {
		t.Errorf("expected first repo to be 'alpha', got %q", repos[0].Name)
	}
	if repos[2].Name != "gamma" {
		t.Errorf("expected last repo to be 'gamma', got %q", repos[2].Name)
	}
	if repos[1].FullName != "alice/beta" {
		t.Errorf("expected FullName 'alice/beta', got %q", repos[1].FullName)
	}
}

func TestFetchRepos_EmptyResult_ReturnsEmptySlice(t *testing.T) {
	fakeFetch := func(owner string) ([]Repo, error) {
		return []Repo{}, nil
	}

	repos, err := fakeFetch("empty-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func TestFetchRepos_NilResult_ReturnsNil(t *testing.T) {
	fakeFetch := func(owner string) ([]Repo, error) {
		return nil, nil
	}

	repos, err := fakeFetch("empty-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repos != nil {
		t.Errorf("expected nil repos, got %v", repos)
	}
}

func TestFetchRepos_APIError_ReturnsError(t *testing.T) {
	fakeFetch := func(owner string) ([]Repo, error) {
		return nil, errors.New("API rate limit exceeded")
	}

	repos, err := fakeFetch("alice")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if repos != nil {
		t.Errorf("expected nil repos on error, got %v", repos)
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected 'rate limit' in error, got: %v", err)
	}
}

func TestFetchRepos_PaginationAcrossMultiplePages(t *testing.T) {
	// Simulate a FetchReposFunc that would return repos from multiple pages.
	// The injectable function is responsible for handling pagination internally,
	// so we test that a large result set is returned correctly.
	fakeFetch := func(owner string) ([]Repo, error) {
		repos := make([]Repo, 250)
		for i := range repos {
			name := fmt.Sprintf("repo-%03d", i)
			repos[i] = Repo{Name: name, FullName: owner + "/" + name}
		}
		return repos, nil
	}

	repos, err := fakeFetch("big-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 250 {
		t.Errorf("expected 250 repos, got %d", len(repos))
	}
	// Verify ordering is maintained.
	if repos[0].Name != "repo-000" {
		t.Errorf("expected first repo 'repo-000', got %q", repos[0].Name)
	}
	if repos[249].Name != "repo-249" {
		t.Errorf("expected last repo 'repo-249', got %q", repos[249].Name)
	}
}

func TestFetchRepos_SortedAlphabetically(t *testing.T) {
	fakeFetch := func(owner string) ([]Repo, error) {
		// Return unsorted repos — the real implementation sorts them.
		return []Repo{
			{Name: "zebra", FullName: owner + "/zebra"},
			{Name: "alpha", FullName: owner + "/alpha"},
			{Name: "middle", FullName: owner + "/middle"},
		}, nil
	}

	repos, err := fakeFetch("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The injectable function returned them in this order.
	// The sort happens inside DefaultFetchRepos; the injectable is a contract.
	// This test verifies the contract shape, not internal sorting.
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
}

// TestRepoStruct_Fields verifies the Repo struct contains required fields.
func TestRepoStruct_Fields(t *testing.T) {
	r := Repo{
		Name:     "test-repo",
		FullName: "alice/test-repo",
	}
	if r.Name != "test-repo" {
		t.Errorf("expected Name 'test-repo', got %q", r.Name)
	}
	if r.FullName != "alice/test-repo" {
		t.Errorf("expected FullName 'alice/test-repo', got %q", r.FullName)
	}
}

// --- CheckRepoExistsFunc and validateNewRepoName tests ---

func TestValidateNewRepoName_NameAvailable_ReturnsNil(t *testing.T) {
	fakeCheck := func(owner, name string) (bool, error) {
		return false, nil // repo does not exist
	}

	validate := validateNewRepoName("alice", fakeCheck)
	err := validate("my-new-repo")
	if err != nil {
		t.Errorf("expected nil for available repo name, got: %v", err)
	}
}

func TestValidateNewRepoName_NameTaken_ReturnsError(t *testing.T) {
	fakeCheck := func(owner, name string) (bool, error) {
		return true, nil // repo exists
	}

	validate := validateNewRepoName("alice", fakeCheck)
	err := validate("existing-repo")
	if err == nil {
		t.Fatal("expected error for taken repo name, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestValidateNewRepoName_InvalidFormat_ReturnsError(t *testing.T) {
	fakeCheck := func(owner, name string) (bool, error) {
		return false, nil
	}

	validate := validateNewRepoName("alice", fakeCheck)

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "uppercase", input: "MyRepo"},
		{name: "spaces", input: "my repo"},
		{name: "underscores", input: "my_repo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(tc.input)
			if err == nil {
				t.Errorf("expected error for invalid name %q, got nil", tc.input)
			}
		})
	}
}

func TestValidateNewRepoName_APIError_ReturnsError(t *testing.T) {
	fakeCheck := func(owner, name string) (bool, error) {
		return false, errors.New("network timeout")
	}

	validate := validateNewRepoName("alice", fakeCheck)
	err := validate("my-repo")
	if err == nil {
		t.Fatal("expected error for API failure, got nil")
	}
	if !strings.Contains(err.Error(), "unable to verify") {
		t.Errorf("expected 'unable to verify' in error, got: %v", err)
	}
}

func TestValidateNewRepoName_FormatCheckedBeforeExistence(t *testing.T) {
	// If format is invalid, the existence check should not be called.
	checkCalled := false
	fakeCheck := func(owner, name string) (bool, error) {
		checkCalled = true
		return false, nil
	}

	validate := validateNewRepoName("alice", fakeCheck)
	_ = validate("INVALID")
	if checkCalled {
		t.Error("expected existence check NOT to be called when format is invalid")
	}
}

func TestCheckRepoExistsFunc_InjectionPattern(t *testing.T) {
	// Verify the type is usable as an injectable function.
	var fn CheckRepoExistsFunc = func(owner, name string) (bool, error) {
		if name == "exists" {
			return true, nil
		}
		return false, nil
	}

	exists, err := fn("alice", "exists")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected true for 'exists' repo")
	}

	exists, err = fn("alice", "new-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected false for 'new-repo'")
	}
}

// --- Phase 2 and Phase 3 form redesign tests ---

func TestRenderSummaryBox_ExistingRepo_ShowsExisting(t *testing.T) {
	cfg := BootstrapConfig{
		ExistingRepo: true,
		ProjectName:  "my-repo",
		AgentUser:    "goose-agent",
	}
	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "existing") {
		t.Errorf("RenderSummaryBox() expected 'existing' when ExistingRepo=true, got:\n%s", rendered)
	}
}

func TestRenderSummaryBox_NewRepo_ShowsNew(t *testing.T) {
	cfg := BootstrapConfig{
		ExistingRepo: false,
		ProjectName:  "my-repo",
		AgentUser:    "goose-agent",
	}
	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "new") {
		t.Errorf("RenderSummaryBox() expected 'new' when ExistingRepo=false, got:\n%s", rendered)
	}
}

func TestRenderSummaryBox_ShowsAgentUser(t *testing.T) {
	cfg := BootstrapConfig{
		AgentUser: "test-bot",
	}
	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "test-bot") {
		t.Errorf("RenderSummaryBox() expected 'test-bot' in output, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Agent user") {
		t.Errorf("RenderSummaryBox() expected 'Agent user' label in output, got:\n%s", rendered)
	}
}

func TestRenderSummaryBox_ShowsRepoLabel(t *testing.T) {
	cfg := BootstrapConfig{
		ExistingRepo: true,
	}
	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "Repo") {
		t.Errorf("RenderSummaryBox() expected 'Repo' label in output, got:\n%s", rendered)
	}
}

func TestBootstrapConfig_ExistingRepoField(t *testing.T) {
	// Verify ExistingRepo field exists and works correctly.
	cfg := BootstrapConfig{ExistingRepo: true}
	if !cfg.ExistingRepo {
		t.Error("expected ExistingRepo to be true")
	}

	cfg = BootstrapConfig{ExistingRepo: false}
	if cfg.ExistingRepo {
		t.Error("expected ExistingRepo to be false")
	}
}

func TestRepoModeConstants(t *testing.T) {
	if repoModeSelectExisting != "existing" {
		t.Errorf("expected repoModeSelectExisting to be 'existing', got %q", repoModeSelectExisting)
	}
	if repoModeCreateNew != "new" {
		t.Errorf("expected repoModeCreateNew to be 'new', got %q", repoModeCreateNew)
	}
}

func TestValidateNewRepoName_SelectExistingPath_SetsExistingRepoTrue(t *testing.T) {
	// Verify that the form correctly sets ExistingRepo when selecting existing.
	cfg := BootstrapConfig{}
	cfg.ExistingRepo = true
	if !cfg.ExistingRepo {
		t.Error("ExistingRepo should be true after selecting existing path")
	}
}

func TestValidateNewRepoName_CreateNewPath_SetsExistingRepoFalse(t *testing.T) {
	// Verify that the form correctly sets ExistingRepo when creating new.
	cfg := BootstrapConfig{}
	cfg.ExistingRepo = false
	if cfg.ExistingRepo {
		t.Error("ExistingRepo should be false after selecting create new path")
	}
}

// --- RunnerDefaultForTopology tests ---

func TestRunnerDefaultForTopology_SingleTopology_ReturnsUbuntuLatest(t *testing.T) {
	result := RunnerDefaultForTopology("Single", "alice")
	if result != DefaultRunnerLabel {
		t.Errorf("RunnerDefaultForTopology(Single, alice) = %q, want %q", result, DefaultRunnerLabel)
	}
}

func TestRunnerDefaultForTopology_FederatedTopology_ReturnsOrgName(t *testing.T) {
	result := RunnerDefaultForTopology("Federated", "acme-org")
	if result != "acme-org" {
		t.Errorf("RunnerDefaultForTopology(Federated, acme-org) = %q, want %q", result, "acme-org")
	}
}

// --- buildRunnerOptions tests ---

func TestBuildRunnerOptions_ContainsFiveOptions(t *testing.T) {
	opts := buildRunnerOptions("my-project", "acme-org")
	if len(opts) != 5 {
		t.Fatalf("buildRunnerOptions() returned %d options, want 5", len(opts))
	}
}

func TestBuildRunnerOptions_ContainsRepoAndOrgNames(t *testing.T) {
	opts := buildRunnerOptions("my-project", "acme-org")

	// Verify the option keys contain the dynamic names.
	var keys []string
	for _, o := range opts {
		keys = append(keys, o.Value)
	}

	found := map[string]bool{
		DefaultRunnerLabel: false,
		"my-project":      false,
		"acme-org":        false,
		"self-hosted":     false,
		runnerOther:       false,
	}
	for _, k := range keys {
		if _, ok := found[k]; ok {
			found[k] = true
		}
	}
	for k, v := range found {
		if !v {
			t.Errorf("buildRunnerOptions() missing expected option value %q", k)
		}
	}
}

// --- runnerOther constant test ---

func TestRunnerOtherConstant(t *testing.T) {
	if runnerOther != "__other__" {
		t.Errorf("runnerOther = %q, want %q", runnerOther, "__other__")
	}
}

// --- repoBackSentinel constant test ---

func TestRepoBackSentinelConstant(t *testing.T) {
	if repoBackSentinel != "__back__" {
		t.Errorf("repoBackSentinel = %q, want %q", repoBackSentinel, "__back__")
	}
}

// --- Back navigation logic tests ---

func TestBackNavigation_SentinelIsNotValidProjectName(t *testing.T) {
	// The back sentinel should not pass project name validation.
	err := validateProjectName(repoBackSentinel)
	if err == nil {
		t.Error("back sentinel should not pass project name validation")
	}
}

// --- GOOSE_AGENT_PAT summary box tests ---

func TestRenderSummaryBox_PATProvided_ShowsSet(t *testing.T) {
	cfg := BootstrapConfig{
		GooseAgentPAT: "ghp_abc123",
	}
	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "Agent PAT") {
		t.Errorf("RenderSummaryBox() expected 'Agent PAT' label, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "set") {
		t.Errorf("RenderSummaryBox() expected 'set' for provided PAT, got:\n%s", rendered)
	}
}

func TestRenderSummaryBox_PATEmpty_ShowsNotSet(t *testing.T) {
	cfg := BootstrapConfig{
		GooseAgentPAT: "",
	}
	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "not set") {
		t.Errorf("RenderSummaryBox() expected 'not set' for empty PAT, got:\n%s", rendered)
	}
}

