package mount

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Pipeline step-ordering invariants.
//
// This test parses .github/workflows/agentic-pipeline.yml and walks each
// job's steps in order, modelling which tools and resources are available
// after each step. It then asserts that every step's preconditions are
// satisfied — catching the class of bug that v2.8.11 → v2.8.13 kept
// re-introducing:
//
//   - A composite action referenced before the .agents/ submodule was
//     checked out (`Can't find action.yml`).
//   - `gh` invoked before install-ai-tools installed it (self-hosted
//     runners don't have gh pre-installed) (`gh: command not found`).
//   - `goose` invoked before install-ai-tools installed it.
//
// The model deliberately assumes the WORST baseline: a bare self-hosted
// runner with only the universal POSIX toolchain (bash, curl, jq, git,
// sed, grep). Anything else must be explicitly installed by a step
// before it can be invoked. This keeps the workflow portable across
// GitHub-hosted ubuntu-latest (which has more pre-installed) and
// minimal self-hosted runners (which have less).

type workflowState struct {
	// True once a composite action under .agents/.github/actions/... can
	// be safely referenced.
	agentsSubmoduleOnDisk bool
	// True once install-ai-tools (or any step that installs it) has run.
	ghInstalled    bool
	gooseInstalled bool
}

type pipelineJob struct {
	Name  string         `yaml:"name"`
	Steps []pipelineStep `yaml:"steps"`
}

type pipelineStep struct {
	Name string         `yaml:"name"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
}

type pipelineWorkflow struct {
	Jobs map[string]pipelineJob `yaml:"jobs"`
}

// universal commands present on every Linux runner — bare-image baseline.
// Anything not in this set must be installed by a prior step.
var universalCommands = map[string]bool{
	"bash":   true,
	"sh":     true,
	"curl":   true,
	"wget":   true,
	"jq":     true,
	"git":    true,
	"sed":    true,
	"grep":   true,
	"awk":    true,
	"tr":     true,
	"cat":    true,
	"echo":   true,
	"head":   true,
	"tail":   true,
	"cut":    true,
	"sort":   true,
	"uniq":   true,
	"mkdir":  true,
	"rm":     true,
	"cp":     true,
	"mv":     true,
	"chmod":  true,
	"base64": true,
	"date":   true,
	"sleep":  true,
	"exit":   true,
	"set":    true,
	"printf": true,
}

// Commands installed by install-ai-tools (which setup-goose-env wraps).
var installedByAITools = map[string]bool{
	"gh":     true, // installed via apt-get gh
	"goose":  true, // installed via curl + tar to ~/.local/bin
	"node":   true, // setup-node action
	"npm":    true, // comes with node
	"claude": true, // npm install -g @anthropic-ai/claude-code
}

// Detects shell command invocations in a `run:` block. We deliberately
// only look at the first token of each (logical) line — finding every
// invocation in arbitrary shell is impossible, but the leading-token
// heuristic catches every real usage in this workflow.
var commandInvocationRe = regexp.MustCompile(`(?m)^[ \t]*(?:[A-Z_][A-Z0-9_]*=\S+[ \t]+)*([a-zA-Z][a-zA-Z0-9_-]*)\b`)

// TestPipelineStepOrdering walks every job's steps in order and asserts
// that each step's tool/resource preconditions hold at the moment it
// runs. This is the regression gate for the v2.8.11 → v2.8.13 incidents.
func TestPipelineStepOrdering(t *testing.T) {
	path := reusableWorkflowPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var wf pipelineWorkflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	if len(wf.Jobs) == 0 {
		t.Fatalf("no jobs parsed from %s — schema drift?", path)
	}

	// Track which jobs we expect — surface drift if a job appears or disappears.
	expectedJobs := []string{
		"feature-design",
		"dev-session",
		"compliance-verify",
		"pr-review-session",
		"issue-session",
		"feature-complete",
	}
	seen := map[string]bool{}
	for jobID := range wf.Jobs {
		seen[jobID] = true
	}
	for _, want := range expectedJobs {
		if !seen[want] {
			t.Errorf("missing expected job %q — pipeline structure changed", want)
		}
	}

	for jobID, job := range wf.Jobs {
		t.Run(jobID, func(t *testing.T) {
			state := workflowState{}
			for i, step := range job.Steps {
				checkStep(t, jobID, i, step, &state)
				updateState(step, &state)
			}
		})
	}
}

// checkStep verifies the step's preconditions against the current state.
func checkStep(t *testing.T, jobID string, idx int, step pipelineStep, state *workflowState) {
	t.Helper()

	// `uses: ./.agents/.github/actions/...` requires the .agents/
	// submodule to be on disk.
	if strings.HasPrefix(strings.TrimSpace(step.Uses), "./.agents/.github/actions/") {
		if !state.agentsSubmoduleOnDisk {
			t.Errorf(
				"job %s, step %d (%q): uses local composite %q but no prior step "+
					"checked out the repo with submodules: recursive. The .agents/ "+
					"directory will be empty and the runner will fail with "+
					"\"Can't find action.yml\". Either move this step after the checkout, "+
					"or inline its logic.",
				jobID, idx, step.Name, step.Uses,
			)
		}
	}

	// `run:` blocks: scan for commands that need to be installed first.
	if step.Run != "" {
		for _, cmd := range commandsUsedIn(step.Run) {
			if universalCommands[cmd] {
				continue
			}
			if !installedByAITools[cmd] {
				continue // unknown command — not our concern
			}
			if cmd == "gh" && !state.ghInstalled {
				t.Errorf(
					"job %s, step %d (%q): invokes %q but install-ai-tools has not "+
						"run yet in this job. Self-hosted runners do not have gh "+
						"pre-installed (it is installed via apt-get by install-ai-tools "+
						"inside setup-goose-env). Move this step after setup-goose-env, "+
						"or replace %q with curl+jq against the REST API.",
					jobID, idx, step.Name, cmd, cmd,
				)
			}
			if cmd == "goose" && !state.gooseInstalled {
				t.Errorf(
					"job %s, step %d (%q): invokes %q but install-ai-tools has not "+
						"run yet in this job. Move this step after setup-goose-env.",
					jobID, idx, step.Name, cmd,
				)
			}
		}
	}
}

// updateState updates the state model based on what the step did.
func updateState(step pipelineStep, state *workflowState) {
	uses := strings.TrimSpace(step.Uses)

	// Any actions/checkout step with submodules: recursive populates
	// .agents/.
	if strings.HasPrefix(uses, "actions/checkout@") {
		if sub, ok := step.With["submodules"]; ok {
			if s, ok := sub.(string); ok && s == "recursive" {
				state.agentsSubmoduleOnDisk = true
			}
		}
	}

	// setup-goose-env wraps install-ai-tools which installs gh and goose
	// (and Node, but we don't track Node usage).
	if strings.HasPrefix(uses, "./.agents/.github/actions/setup-goose-env") {
		state.ghInstalled = true
		state.gooseInstalled = true
	}

	// install-ai-tools called directly installs the same.
	if strings.HasPrefix(uses, "./.agents/.github/actions/install-ai-tools") {
		state.ghInstalled = true
		state.gooseInstalled = true
	}
}

// commandsUsedIn returns the set of leading-token commands seen in a
// shell script. Heuristic — see commandInvocationRe.
func commandsUsedIn(script string) []string {
	matches := commandInvocationRe.FindAllStringSubmatch(script, -1)
	seen := map[string]bool{}
	out := []string{}
	// Common bash keywords / built-ins / control flow we should skip
	// when they appear as a line's leading token.
	skip := map[string]bool{
		"if": true, "elif": true, "else": true, "fi": true,
		"for": true, "while": true, "do": true, "done": true,
		"case": true, "esac": true, "in": true, "then": true,
		"return": true, "break": true, "continue": true,
		"local": true, "declare": true, "readonly": true,
		"function": true, "true": true, "false": true,
		"export": true,
	}
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		cmd := m[1]
		if skip[cmd] {
			continue
		}
		if seen[cmd] {
			continue
		}
		seen[cmd] = true
		out = append(out, cmd)
	}
	return out
}
