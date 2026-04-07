package bootstrap

import (
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// SetPipelineVariables sets the RUNNER_LABEL, GOOSE_PROVIDER, and GOOSE_MODEL
// GitHub Actions repo variables. Each variable failure is non-fatal — a warning
// is logged and the step continues with the remaining variables.
func SetPipelineVariables(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
	fullName := cfg.Owner + "/" + state.RepoName

	vars := []struct {
		name  string
		value string
	}{
		{name: "RUNNER_LABEL", value: cfg.RunnerLabel},
		{name: "GOOSE_PROVIDER", value: cfg.GooseProvider},
		{name: "GOOSE_MODEL", value: cfg.GooseModel},
	}

	for _, v := range vars {
		out, err := run("gh", "variable", "set", v.name, "--body", v.value, "--repo", fullName)
		if err != nil {
			fmt.Fprintln(w, "  "+ui.RenderWarning(fmt.Sprintf("Could not set %s variable: %s", v.name, strings.TrimSpace(out))))
			continue
		}
		fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("· %s=%s set at repo level", v.name, v.value)))
	}

	return nil
}
