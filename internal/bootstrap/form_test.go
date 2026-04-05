package bootstrap

import (
	"errors"
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
		Stack:       "Go",
		Antora:      false,
	}

	rendered := RenderSummaryBox(cfg)

	checks := []string{
		"Single",
		"newopenbss",
		"my-project",
		"A test bench",
		"Go",
		"No",
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

// --- isPersonalSingleTopology tests ---

func TestIsPersonalSingleTopology(t *testing.T) {
	tests := []struct {
		name      string
		topology  string
		ownerType string
		want      bool
	}{
		{
			name:      "personal + single returns true",
			topology:  "Single",
			ownerType: OwnerTypeUser,
			want:      true,
		},
		{
			name:      "personal + federated returns false",
			topology:  "Federated",
			ownerType: OwnerTypeUser,
			want:      false,
		},
		{
			name:      "org + single returns false",
			topology:  "Single",
			ownerType: OwnerTypeOrg,
			want:      false,
		},
		{
			name:      "org + federated returns false",
			topology:  "Federated",
			ownerType: OwnerTypeOrg,
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isPersonalSingleTopology(tc.topology, tc.ownerType)
			if got != tc.want {
				t.Errorf("isPersonalSingleTopology(%q, %q) = %v, want %v", tc.topology, tc.ownerType, got, tc.want)
			}
		})
	}
}
