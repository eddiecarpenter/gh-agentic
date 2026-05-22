package mount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Layer 4 — structural and contract tests for composite actions and
// for each job's `if:` trigger expression.
//
// These tests do not execute scripts. They verify shape:
//   - Each composite action has a name, description, runs.using:
//     composite, and at least one step.
//   - Every input the workflow passes to a composite action is
//     declared in that action's inputs:.
//   - Every job's `if:` trigger expression contains the substrings
//     we expect for the event it claims to handle, so a typo in
//     the gate is caught at test time.

type compositeAction struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Inputs      map[string]any `yaml:"inputs"`
	Runs        struct {
		Using string `yaml:"using"`
		Steps []any  `yaml:"steps"`
	} `yaml:"runs"`
}

// TestCompositeActionsAreWellFormed — every composite has a name,
// description, runs.using == composite, and at least one step.
func TestCompositeActionsAreWellFormed(t *testing.T) {
	actionsDir := compositeActionsDir(t)
	entries, err := os.ReadDir(actionsDir)
	if err != nil {
		t.Fatalf("read %s: %v", actionsDir, err)
	}
	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		actionYAML := filepath.Join(actionsDir, e.Name(), "action.yml")
		data, err := os.ReadFile(actionYAML)
		if err != nil {
			continue
		}
		found++
		var ca compositeAction
		if err := yaml.Unmarshal(data, &ca); err != nil {
			t.Errorf("%s parse error: %v", e.Name(), err)
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			if strings.TrimSpace(ca.Name) == "" {
				t.Errorf("composite %s has empty name:", e.Name())
			}
			if strings.TrimSpace(ca.Description) == "" {
				t.Errorf("composite %s has empty description:", e.Name())
			}
			if ca.Runs.Using != "composite" {
				t.Errorf("composite %s: runs.using = %q, want \"composite\"", e.Name(), ca.Runs.Using)
			}
			if len(ca.Runs.Steps) == 0 {
				t.Errorf("composite %s: no steps declared", e.Name())
			}
		})
	}
	if found == 0 {
		t.Fatalf("no composite actions discovered under %s", actionsDir)
	}
}

// TestCompositeActionInputsMatchCallSites — for every `uses:
// ./.agents/.github/actions/<name>` call site in the reusable
// workflow, verify every key under `with:` is declared in that
// composite's `inputs:`. Catches typos like passing `agentic-version`
// when the composite expects `agentic-framework-version`.
func TestCompositeActionInputsMatchCallSites(t *testing.T) {
	wfPath := reusableWorkflowPath(t)
	data, err := os.ReadFile(wfPath)
	if err != nil {
		t.Fatalf("read %s: %v", wfPath, err)
	}
	var wf map[string]any
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("parse workflow: %v", err)
	}

	// Load every composite's input list.
	actionInputs := map[string]map[string]bool{}
	actionsDir := compositeActionsDir(t)
	entries, _ := os.ReadDir(actionsDir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ap := filepath.Join(actionsDir, e.Name(), "action.yml")
		ad, err := os.ReadFile(ap)
		if err != nil {
			continue
		}
		var ca compositeAction
		if err := yaml.Unmarshal(ad, &ca); err != nil {
			continue
		}
		ins := map[string]bool{}
		for k := range ca.Inputs {
			ins[k] = true
		}
		actionInputs[e.Name()] = ins
	}

	// Walk every job's steps and check each uses: call site.
	jobs := wf["jobs"].(map[string]any)
	for jobID, j := range jobs {
		job := j.(map[string]any)
		steps, _ := job["steps"].([]any)
		for i, s := range steps {
			step := s.(map[string]any)
			uses, _ := step["uses"].(string)
			uses = strings.TrimSpace(uses)
			const prefix = "./.agents/.github/actions/"
			if !strings.HasPrefix(uses, prefix) {
				continue
			}
			actionName := strings.TrimPrefix(uses, prefix)
			declared, ok := actionInputs[actionName]
			if !ok {
				t.Errorf("job %s step %d uses unknown composite %q", jobID, i, actionName)
				continue
			}
			with, _ := step["with"].(map[string]any)
			for k := range with {
				if !declared[k] {
					t.Errorf(
						"job %s step %d (uses: %s): passes input %q that the composite "+
							"does not declare. Declared inputs: %v",
						jobID, i, uses, k, sortedKeys(declared),
					)
				}
			}
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// simple insertion sort; the lists are short
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// TestJobTriggersMatchEvents — assert that every job's `if:`
// expression contains the GitHub event name and condition it claims
// to handle. This is a smoke test against typos like
// `github.event.action == 'labelled'`.
func TestJobTriggersMatchEvents(t *testing.T) {
	wfPath := reusableWorkflowPath(t)
	data, err := os.ReadFile(wfPath)
	if err != nil {
		t.Fatalf("read %s: %v", wfPath, err)
	}
	var wf map[string]any
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	jobs := wf["jobs"].(map[string]any)

	expected := map[string][]string{
		// jobID → substrings every `if:` must contain
		"feature-design": {
			"github.event_name == 'issues'",
			"github.event.action == 'labeled'",
			"'in-design'",
		},
		"dev-session": {
			"github.event_name == 'issues'",
			"github.event.action == 'labeled'",
			"'in-development'",
		},
		"compliance-verify": {
			"'in-verification'",
			"github.event.action == 'labeled'",
		},
		"pr-review-session": {
			"pull_request_review",
			"changes_requested",
			"feature/", // head ref filter
		},
		"issue-session": {
			"github.event_name == 'issues'",
			"github.event.action == 'labeled'",
			"'assigned-to-agent'",
		},
		"feature-complete": {
			"github.event_name == 'pull_request'",
			"github.event.pull_request.merged == true",
			"feature/", // head ref filter
		},
	}

	for jobID, mustContain := range expected {
		t.Run(jobID, func(t *testing.T) {
			job, ok := jobs[jobID].(map[string]any)
			if !ok {
				t.Fatalf("job %s missing", jobID)
			}
			ifExpr, _ := job["if"].(string)
			if ifExpr == "" {
				t.Fatalf("job %s has no `if:` — would run on every event", jobID)
			}
			// Normalise whitespace for substring matching.
			normalised := strings.Join(strings.Fields(ifExpr), " ")
			for _, sub := range mustContain {
				normSub := strings.Join(strings.Fields(sub), " ")
				if !strings.Contains(normalised, normSub) {
					t.Errorf(
						"job %s `if:` expression is missing required substring %q.\nFull if: %q",
						jobID, sub, ifExpr,
					)
				}
			}
		})
	}
}
