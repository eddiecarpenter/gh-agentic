package mount

import "strings"

// claudeMDTemplate is the standard CLAUDE.md content for v2 domain repos.
const claudeMDTemplate = `# CLAUDE.md

@AGENTS.md
`

// agentsMDTemplate is the standard AGENTS.md content for v2 domain repos.
// It references the mounted .ai/ framework and includes the bootstrap rule.
const agentsMDTemplate = `# AGENTS.md

@.ai/RULEBOOK.md
@LOCALRULES.md

## Bootstrap Rule

If the .ai/ directory does not exist, stop immediately.

- **Interactive context:** Instruct the user to run:
  ` + "`" + `gh agentic -v2 mount $(cat .ai-version)` + "`" + `

- **CI context:** Fail with the message:
  "Framework not mounted. Add a mount step before the pipeline:
  ` + "`" + `gh agentic -v2 mount $(cat .ai-version)` + "`" + `"

Do not proceed with any other work until the framework is mounted.
`

// workflowTemplate returns the agentic-pipeline wrapper workflow content
// for a domain repo. The version parameter is substituted into the uses: tag.
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
    uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline-reusable.yml@{{VERSION}}
    secrets: inherit
    permissions:
      contents: write
      issues: write
      pull-requests: write
`, "{{VERSION}}", version)
}

// releaseWorkflowTemplate returns the release wrapper workflow content
// for a domain repo. The version parameter is substituted into the uses: tag.
func releaseWorkflowTemplate(version string) string {
	return strings.ReplaceAll(`name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    uses: eddiecarpenter/gh-agentic/.github/workflows/release-reusable.yml@{{VERSION}}
    secrets: inherit
    permissions:
      contents: write
`, "{{VERSION}}", version)
}
