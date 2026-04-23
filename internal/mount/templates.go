package mount

import "strings"

// claudeMDTemplate is the standard CLAUDE.md content for domain repos.
const claudeMDTemplate = `# CLAUDE.md

@AGENTS.md
`

// agentsMDTemplate is the standard AGENTS.md content for domain repos.
// It references the mounted .ai/ framework and includes the bootstrap rule.
const agentsMDTemplate = `# AGENTS.md

@.ai/RULEBOOK.md
@LOCALRULES.md

## Bootstrap Rule

If the .ai/ directory does not exist, stop immediately.

- **Interactive context:** Instruct the user to run:
  ` + "`" + `gh agentic mount` + "`" + `

- **CI context:** Fail with the message:
  "Framework not mounted. Add a mount step before the pipeline:
  ` + "`" + `gh agentic mount` + "`" + `"

Do not proceed with any other work until the framework is mounted.
`

// workflowTemplate returns the agentic-pipeline wrapper workflow content
// for a domain repo. The version parameter is substituted into the uses: tag.
//
// The wrapper subscribes to the domain repo's local events and forwards
// them via `uses:` into the consolidated framework workflow. The
// `workflow_dispatch` inputs (pr_number, branch_name) are forwarded
// through the `with:` block — without that forwarding, the PR-review-
// session branch of the reusable sees empty strings and fails.
func workflowTemplate(version string) string {
	return strings.ReplaceAll(`name: Agentic Pipeline

on:
  issues:
    types: [labeled, assigned]
  pull_request:
    types: [closed]
    branches:
      - main
  pull_request_review:
    types: [submitted]
  workflow_dispatch:
    inputs:
      pr_number:
        description: 'PR number to process review comments for'
        required: true
        type: string
      branch_name:
        description: 'Feature branch the PR is open on'
        required: true
        type: string

jobs:
  pipeline:
    uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline.yml@{{VERSION}}
    with:
      pr_number: ${{ inputs.pr_number || '' }}
      branch_name: ${{ inputs.branch_name || '' }}
    secrets: inherit
    permissions:
      contents: write
      issues: write
      pull-requests: write
`, "{{VERSION}}", version)
}

// releaseWorkflowTemplate returns the release wrapper workflow content
// for a domain repo. The version parameter is substituted into the uses: tag.
//
// Triggers on `release: published` — the framework-provided workflow
// handles AI release-note generation and updates the release body. Release
// CREATION (GoReleaser, npm publish, cargo publish, etc.) is each domain
// repo's own concern and lives in a separate workflow of its own.
func releaseWorkflowTemplate(version string) string {
	return strings.ReplaceAll(`name: Release

on:
  release:
    types: [published]

jobs:
  release:
    uses: eddiecarpenter/gh-agentic/.github/workflows/release.yml@{{VERSION}}
    secrets: inherit
    permissions:
      contents: write
`, "{{VERSION}}", version)
}
