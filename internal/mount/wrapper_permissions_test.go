package mount

import (
	"os"
	"strings"
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
// caller wrapper (runtime workflowTemplate in templates.go) must grant at least the
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
	tmpl := []byte(workflowTemplate("v1.2.3"))
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

// secretsWorkflow parses workflow_call.secrets (reusable) and jobs.pipeline.secrets
// (caller template) for the secret-passing invariant test below.
type secretsReusable struct {
	On struct {
		WorkflowCall struct {
			Secrets map[string]struct {
				Required bool `yaml:"required"`
			} `yaml:"secrets"`
		} `yaml:"workflow_call"`
	} `yaml:"on"`
}

type secretsTemplate struct {
	Jobs map[string]struct {
		Secrets map[string]string `yaml:"secrets"`
	} `yaml:"jobs"`
}

// TestWrapperTemplatePassesDeclaredSecrets guards the cross-account secrets gap
// (v3.0.2): the wrapper template must pass secrets *explicitly* (not `secrets:
// inherit`, which forwards nothing across accounts) and must cover every secret
// the reusable workflow declares as required — while passing only secrets the
// reusable workflow actually declares (an undeclared secret errors at the call).
func TestWrapperTemplatePassesDeclaredSecrets(t *testing.T) {
	reusable, err := os.ReadFile(reusableWorkflowPath(t))
	if err != nil {
		t.Fatalf("read reusable workflow: %v", err)
	}
	var rw secretsReusable
	if err := yaml.Unmarshal(reusable, &rw); err != nil {
		t.Fatalf("parse reusable workflow: %v", err)
	}
	declared := rw.On.WorkflowCall.Secrets
	if len(declared) == 0 {
		t.Fatal("no workflow_call.secrets parsed from reusable workflow — test is a no-op")
	}

	tmpl := []byte(workflowTemplate("v1.2.3"))
	for _, line := range strings.Split(string(tmpl), "\n") {
		if strings.TrimSpace(line) == "secrets: inherit" {
			t.Error("wrapper template uses `secrets: inherit` — it does not forward across accounts; pass secrets explicitly")
		}
	}
	var ct secretsTemplate
	if err := yaml.Unmarshal(tmpl, &ct); err != nil {
		t.Fatalf("parse wrapper template (the {{.Version}} / ${{ }} scalars must stay valid YAML): %v", err)
	}
	passed := ct.Jobs["pipeline"].Secrets
	if len(passed) == 0 {
		t.Fatal("wrapper template's pipeline job passes no secrets explicitly")
	}

	// Every REQUIRED declared secret must be passed.
	for name, d := range declared {
		if d.Required {
			if _, ok := passed[name]; !ok {
				t.Errorf("reusable workflow requires secret %q but the wrapper template does not pass it", name)
			}
		}
	}
	// Every passed secret must be DECLARED (else the reusable-workflow call errors).
	for name := range passed {
		if _, ok := declared[name]; !ok {
			t.Errorf("wrapper template passes secret %q that the reusable workflow does not declare — the call will error", name)
		}
	}
}
