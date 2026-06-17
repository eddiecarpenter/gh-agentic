package mount

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// permRank orders GitHub Actions permission levels: none < read < write.
var permRank = map[string]int{"none": 0, "read": 1, "write": 2, "": 0}

type permWorkflow struct {
	Jobs map[string]struct {
		Permissions map[string]string `yaml:"permissions"`
	} `yaml:"jobs"`
}

// TestWrapperTemplateGrantsNestedJobPermissions guards the bug that caused the
// OpenBSS federation pipeline to fail with startup_failure: the generated
// caller wrapper (templates/agentic-pipeline.yml.tmpl) must grant at least the
// union of every nested job's permissions in the reusable workflow. A called
// reusable-workflow job cannot request more than the caller grants, so a
// missing scope (historically `actions: read`) makes the whole call fail to
// load before any job runs.
func TestWrapperTemplateGrantsNestedJobPermissions(t *testing.T) {
	// Required = the max level each scope is requested at across all nested jobs.
	reusable, err := os.ReadFile(reusableWorkflowPath(t))
	if err != nil {
		t.Fatalf("read reusable workflow: %v", err)
	}
	var rw permWorkflow
	if err := yaml.Unmarshal(reusable, &rw); err != nil {
		t.Fatalf("parse reusable workflow: %v", err)
	}
	required := map[string]string{}
	for jobID, job := range rw.Jobs {
		for scope, level := range job.Permissions {
			if _, ok := permRank[level]; !ok {
				t.Fatalf("job %s: unknown permission level %q for %q", jobID, level, scope)
			}
			if permRank[level] > permRank[required[scope]] {
				required[scope] = level
			}
		}
	}
	if len(required) == 0 {
		t.Fatal("no job-level permissions parsed from reusable workflow — test is a no-op")
	}

	// Granted = the caller wrapper's permissions block.
	tmpl, err := os.ReadFile(repoRelativePath(t, "internal", "mount", "templates", "agentic-pipeline.yml.tmpl"))
	if err != nil {
		t.Fatalf("read wrapper template: %v", err)
	}
	var cw permWorkflow
	if err := yaml.Unmarshal(tmpl, &cw); err != nil {
		t.Fatalf("parse wrapper template (the {{.Version}} / ${{ }} scalars must stay valid YAML): %v", err)
	}
	caller, ok := cw.Jobs["pipeline"]
	if !ok {
		t.Fatal("wrapper template has no 'pipeline' job")
	}

	for scope, need := range required {
		got := caller.Permissions[scope]
		if permRank[got] < permRank[need] {
			t.Errorf("wrapper template grants %q=%q but a nested job requires %q=%q — "+
				"the reusable-workflow call will fail to load (startup_failure). "+
				"Add `%s: %s` to the template's permissions block.",
				scope, got, scope, need, scope, need)
		}
	}
}
