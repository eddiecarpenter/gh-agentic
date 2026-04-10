package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RequiredView defines a view that must exist on the GitHub Project.
type RequiredView struct {
	Name   string `json:"name"`
	Layout string `json:"layout"` // TABLE_LAYOUT or BOARD_LAYOUT
	Filter string `json:"filter"` // GitHub Projects filter string (empty = no filter)
}

// statusFieldWrapper is the nested structure used in the statusField.options format.
type statusFieldWrapper struct {
	Options []StatusOption `json:"options"`
}

// ProjectTemplate represents the structure of base/project-template.json.
// Supports both the legacy top-level statusOptions format and the current
// statusField.options format — whichever is present is used.
type ProjectTemplate struct {
	StatusOptions []StatusOption     `json:"statusOptions"`
	StatusField   statusFieldWrapper `json:"statusField"`
	RequiredViews []RequiredView     `json:"requiredViews"`
}

// ResolvedStatusOptions returns StatusOptions from whichever field is populated.
// Supports both the legacy top-level statusOptions format and the current
// statusField.options format.
func (t *ProjectTemplate) ResolvedStatusOptions() []StatusOption {
	if len(t.StatusOptions) > 0 {
		return t.StatusOptions
	}
	return t.StatusField.Options
}

// LoadProjectTemplate reads and parses .ai/project-template.json from the given root directory.
// Falls back to base/project-template.json for repos not yet migrated to v1.5.0+.
// Returns structured data or a descriptive error if the file is missing or malformed.
//
// TODO(deprecated): remove base/ fallback in next major version.
func LoadProjectTemplate(root string) (*ProjectTemplate, error) {
	path := filepath.Join(root, ".ai", "project-template.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// TODO(deprecated): remove in next major version — base/ fallback for pre-v1.5.0 repos.
		path = filepath.Join(root, "base", "project-template.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading project template %s: %w", path, err)
	}

	var tmpl ProjectTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parsing project template %s: %w", path, err)
	}

	return &tmpl, nil
}
