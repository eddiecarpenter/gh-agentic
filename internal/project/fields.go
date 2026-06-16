package project

import "fmt"

// TargetRepoFieldName is the name of the ProjectV2 TEXT field that records, on a
// control-plane Feature issue, the domain repo the Feature targets. The owner is
// always the control-plane owner; the field holds the bare repo name. (#872)
const TargetRepoFieldName = "Target repo"

// FindField returns the id of the field whose name matches (case-sensitive on
// the ProjectV2 field name) and whether it was found.
func FindField(fields []ProjectField, name string) (string, bool) {
	for _, f := range fields {
		if f.Name == name {
			return f.ID, true
		}
	}
	return "", false
}

// EnsureTargetRepoField ensures the project has a "Target repo" TEXT field,
// creating it if absent. It returns whether it created the field and the field
// id. The fetch/create functions are injected so the same logic serves project
// creation (project.Deps) and doctor repair (doctor.CheckDeps).
func EnsureTargetRepoField(projectID string, fetch FetchProjectFieldsFunc, create CreateProjectFieldFunc) (created bool, fieldID string, err error) {
	fields, err := fetch(projectID)
	if err != nil {
		return false, "", fmt.Errorf("fetching project fields: %w", err)
	}
	if id, ok := FindField(fields, TargetRepoFieldName); ok {
		return false, id, nil
	}
	id, err := create(projectID, TargetRepoFieldName, "TEXT")
	if err != nil {
		return false, "", fmt.Errorf("creating %q field: %w", TargetRepoFieldName, err)
	}
	return true, id, nil
}
