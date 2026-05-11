# Skill Catalogue

Generated from skill frontmatter. Do not edit by hand — regenerate by reading every `skills/*/SKILL.md` and extracting `name`, `description`, and `triggers`.

## Session skills

- **compliance-verify** — Verifies that the implementation on a feature branch satisfies all acceptance criteria from the Feature issue. Evaluates each AC against the actual diff, posts a structured verdict, and either applies `compliance-verified` (all ACs pass — workflow then opens the PR) or posts a `<!-- compliance-feedback:v1 -->` comment and swaps `in-verification` back to `in-development` (triggering a new dev session). Use when GitHub Actions fires on a feature issue labelled `in-verification`. Headless only.
  Triggers: automated

- **dev-session** — Implements a Feature's tasks in order — reading the rationale comment and the ordered Task sub-issues, then walking each open task (read body, implement with reuse discipline, run tests, commit with the prescribed message format, push, close the task issue) until all tasks are done. On exit, the surrounding GitHub Actions workflow applies `in-verification`, triggering compliance-verify. Use when workflow automation fires on in-development label apply against a Feature whose feature-design phase has produced a rationale, ordered tasks, and a feature branch. Headless only; humans running implementation interactively use this skill as a guide.
  Triggers: automated

- **feature-design** — Designs a Feature by producing a rationale artefact (the Design Plan), creating ordered Task sub-issues, and creating the feature branch. Runs headless when invoked against a Feature carrying in-design, or interactively in foreground when invoked against interactive-design. Headless flow auto-triggers implementation at end-of-flow; interactive flow asks the human to choose trigger-now / park-at-designed / cancel. Use when workflow automation fires on in-design label apply, or when a human is running design interactively on a Feature flagged interactive-design.
  Triggers: hybrid

- **foreground-recovery** — Diagnoses and recovers stuck pipeline state interactively — stale concurrency beacons (design-in-progress / development-in-progress / issue-in-progress), label-vs-status mismatches, partial design or dev artefacts on a Feature, orphan feature branches, and backwards-transitioned issues. Walks the human through what's wrong, proposes a remediation, and applies it only after explicit confirmation. Use when the human suspects the pipeline is stuck on a specific Requirement / Feature / issue, or wants to scan the whole project for known stuck-state patterns.
  Triggers: human-interactive

- **issue-session** — Handles a GitHub issue that has been assigned to the agent (label `assigned-to-agent`) — either by answering it as a question (reply + close), implementing it as a small fix (branch + commit + push + PR), or redirecting it as out-of-scope (apply `needs-scoping` and surface a pointer to the pipeline). Headless only. Use when GitHub Actions fires on an `assigned-to-agent` label being applied to a non-pipeline issue.
  Triggers: automated

- **pr-review-session** — Processes PR reviewer feedback by reading unaddressed reviewer comments on a Feature's PR — both inline review comments and general PR comments — and acting on each: pure questions get a direct reply, change requests get implemented (with the commit-discipline applied) and committed in a single batched commit then pushed. Approvals do NOT trigger this skill. Use when GitHub Actions fires on a review submitted with state changes_requested or commented, on a new issue comment on a PR, or on a new pull-request review comment.
  Triggers: automated

- **requirement-scoping** — Decomposes a Requirement into one or more well-formed Feature issues through a conversational, agent-led artefact walk — exploration, framing, MVP, decomposition, acceptance criteria, interactive-design triage, deployment, parking lot — and triggers selected Features for design via the in-design or interactive-design label. Use when a human wants to scope a Requirement that has reached backlog into Features.
  Triggers: human-interactive

- **requirements-session** — Captures a new business need as a Requirement GitHub issue through a conversational, agent-led interview — listening, challenging vague or solution-framed input, and confirming the result with the human before creating the issue. Use when a human wants to record a new business need, idea, or enhancement request as a Requirement.
  Triggers: human-interactive

- **session-init** — Bootstraps the project environment at the start of every session — builds the skill index in memory, runs the gh agentic health check, surfaces the framework state as a greeting, and routes the user to the appropriate phase skill via an interactive menu. Use when a new session starts (RULEBOOK.md mandates this as the first action of every session).
  Triggers: automated

- **solution-architecture** — Creates or extends the project's Foundation Solution Architecture document (docs/ARCHITECTURE.md) through a conversational, agent-led interview — vision, capability domains, system context, architectural decisions, NFRs, integration points, data model, evolution notes. Operates on the current branch (refuses to run on main); the human pushes and opens a PR manually.
  Triggers: human-interactive

- **ux-design** — Creates and extends a project's canonical UX specification at docs/UX_DESIGN.md — a long-lived, governed artefact sibling to docs/ARCHITECTURE.md that defines the project's UX rules, decision-axis verdicts, sanctioned deviations, and standards. Runs at project birth (init mode), on demand to extend rules (extend mode), and from feature-design's interactive flow when a deviation is decided (add-deviation mode).
  Triggers: human

## Operation skills

- **recipe-creation** — Creates or updates a Goose recipe under a thin-shell discipline that prevents recipes from duplicating skill content. Validates that recipe instructions are a one-line pointer at the canonical SKILL.md and refuses to write recipe YAML containing numbered steps, inline gh / git commands, decision logic, or any other playbook content.
  Triggers: human-interactive

- **release-notes** — Generates human-readable, well-structured release notes from the git commit history between the current tag and the previous one, then updates the existing GitHub release body with the AI-written notes. Categorises commits as Features / Fixes / Documentation / Chores; detects newly-added migration docs and prepends a `## ⚠️ Required Action` callout linking each one.
  Triggers: automated

- **skill-creator** — Creates a new skill in this framework that conforms to the skill-spec — frontmatter, sections, verification, error handling, and a minimal evaluation set. Use when the user asks to create a skill, wants to formalise a recurring action as a reusable skill, or is refactoring an existing skill to match the current skill-spec.
  Triggers: human-interactive

## Primitive skills

- **apply-label** — Applies one or more labels to a GitHub issue or pull request and optionally removes conflicting labels in the same call so phase-state transitions are atomic from the caller's perspective. Returns the resulting label set so the caller can verify without a second round-trip.
  Triggers: automated

- **gh-agentic** — Picks the right gh agentic CLI command (with the right flags, especially --raw) to answer framework, project, requirement, feature, or pipeline questions — and runs it. The cheapest token-cost path to read framework state.
  Triggers: automated

- **post-issue-comment** — Posts a comment to a GitHub issue or pull request with the body the caller supplies, and returns the URL and ID of the created comment so the caller can verify or reference it.
  Triggers: automated

- **prompt-user** — Asks the human a single question through the best available UI primitive — Claude Code's structured AskUserQuestion card when running interactively, or an inline conversation prompt when running headlessly — and returns the reply as a structured value to the calling skill.
  Triggers: automated

- **set-issue-status** — Sets the Status field on a GitHub ProjectV2 item for a given issue, finding or creating the project item if needed and resolving the target status name to its option ID at runtime so the caller does not have to deal with GraphQL plumbing.
  Triggers: automated

- **trigger-design** — Triggers the design phase for a Feature by applying the appropriate trigger label (in-design for headless or interactive-design for foreground, based on the needs-interactive-design classification on the Feature) and transitioning the project status to In Design.
  Triggers: automated

- **trigger-implementation** — Triggers the implementation phase for a Feature by removing whichever design-phase label it currently carries (in-design, interactive-design, or designed) and applying in-development, then transitioning the project status to In Development.
  Triggers: automated
