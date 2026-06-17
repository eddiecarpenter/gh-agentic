package project

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// systemDocTemplates are the federated-tier documentation files a control plane
// scaffolds on creation (#875), per concepts/knowledge-plane.md: SYSTEM_BRIEF
// answers "why the system as a whole exists" and SYSTEM_ARCHITECTURE answers
// "the seams between repos". They are written only when absent.
var systemDocTemplates = map[string]string{
	"docs/SYSTEM_BRIEF.md":        systemBriefTemplate,
	"docs/SYSTEM_ARCHITECTURE.md": systemArchitectureTemplate,
}

// ScaffoldFederation scaffolds the control plane's federation artefacts: an
// empty-but-valid FEDERATION.md (no domains registered yet — domains are added
// later via `gh agentic project join`) and the federated-tier system docs. Both
// are write-if-absent, so re-running never clobbers existing content.
func ScaffoldFederation(w io.Writer, root string) error {
	if !IsFederationRepo(root) {
		if err := WriteFederation(root, &Federation{}); err != nil {
			return fmt.Errorf("scaffolding FEDERATION.yaml: %w", err)
		}
		fmt.Fprintf(w, "  %s  FEDERATION.yaml scaffolded (no domains registered yet)\n", ui.StatusOK.Render("✓"))
	}

	for path, tmpl := range systemDocTemplates {
		full := filepath.Join(root, path)
		if _, err := os.Stat(full); err == nil {
			continue // never overwrite an existing doc
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("scaffolding %s: %w", path, err)
		}
		if err := os.WriteFile(full, []byte(tmpl), 0o644); err != nil {
			return fmt.Errorf("scaffolding %s: %w", path, err)
		}
		fmt.Fprintf(w, "  %s  %s scaffolded\n", ui.StatusOK.Render("✓"), path)
	}
	fmt.Fprintf(w, "  Register domain repos from here with: gh agentic project join <owner/repo> --domain <name>\n")
	return nil
}

const systemBriefTemplate = `# SYSTEM_BRIEF.md

> Federated-tier brief (control plane). Answers **why the system as a whole
> exists** — the system-level need, not how any one repo works internally.
> Domain-level briefs live under docs/domains/<domain>/.

## What this system is

<One paragraph: the system this federation delivers, in business terms.>

## Why it exists

<The system-level need. What problem does the whole federation solve that no
single domain repo solves alone?>

## Domains

<Each domain in FEDERATION.md, one line on what it owns. Keep in step with the
manifest as domains are registered via 'gh agentic project join'.>
`

const systemArchitectureTemplate = `# SYSTEM_ARCHITECTURE.md

> Federated-tier architecture (control plane). Answers **the seams between
> repos** — cross-cutting concerns and the contracts that join domains. It must
> never describe how a single repo works internally (that is the domain's own
> ARCHITECTURE.md under docs/domains/<domain>/).

## System context

<The major domains and how they relate at the seams — events, APIs, shared
contracts. A diagram or bullet list of the integration points.>

## Cross-cutting concerns

<Concerns that span domains: auth, observability, shared schemas, etc.>

## Contracts between domains

<The interfaces domains depend on across the seam — event schemas, API
boundaries, shared data shapes.>
`
