# Skill Catalogue

Generated from skill frontmatter by skills/build-catalogue.md. Do not edit by hand.

## Session skills

- **dev-session** — Implements every open Task sub-issue on the feature branch in order, commits per task, verifies acceptance criteria coverage, and exits cleanly so the workflow can open the PR. Use when GitHub Actions triggers this session automatically on a Feature issue receiving the in-development label — never run interactively.
  Triggers: automation: in-development

- **feature-design** — Decomposes a Feature issue into ordered Task sub-issues that cover every acceptance criterion, creates the feature branch, and hands off to Dev Session via the in-development label. Use when GitHub Actions triggers this session automatically on a Feature issue receiving the in-design label — never run interactively.
  Triggers: automation: in-design

- **requirement-scoping** — Decomposes a Requirement issue into one or more well-formed Feature issues with acceptance criteria, UX triage, and deployment strategy, and hands selected features to Feature Design via the in-design label. Use when a human invokes the requirement-scoping skill to scope a backlog requirement into features.
  Triggers: human-interactive

- **issue-session** — Handles a GitHub Issue assigned to the agent — routes by label to either fix a bug on a new branch or answer a question as a comment, and exits cleanly so the workflow can open a PR if code changed. Use when GitHub Actions triggers this session automatically on an issue being assigned to the agent user — never run interactively.
  Triggers: automation: issue-assigned

- **pr-review-session** — Processes inline review comments on a PR — answers questions, implements change requests with tests, and escalates ambiguous or scope-changing feedback via the needs-foreground-review label. Use when GitHub Actions triggers this session automatically on a PR review being submitted — never run interactively.
  Triggers: automation: pr-review-submitted

- **requirements-session** — Captures a new business need as a Requirement issue in GitHub and, when the scope is clear, completes Requirement Scoping inline. Use when a human invokes the requirements-session skill to record a new idea, need, or enhancement request.
  Triggers: human-interactive

## Recovery skills

- **foreground-recovery** — The human-driven escape hatch for any blocked pipeline state — diagnoses failures from exact error output, applies minimal fixes, and optionally rewinds to an earlier pipeline phase with explicit human confirmation. Use when the automated pipeline is blocked (red build, failing tests, merge conflict, silent workflow failure, or any situation requiring manual intervention) and a human opens the Foreground Recovery recipe.
  Triggers: human-interactive

## Bootstrap skills

- **post-sync** — Handles post-sync upgrade actions left behind in POST_SYNC.md — runs required migration steps (commands, config changes, file renames) in interactive sessions, or warns and exits in automated sessions. Use only when session-init detects POST_SYNC.md at the repo root — never invoke directly.
  Triggers: post-sync

- **session-init** — Loads the project environment at the start of every session — reads the project brief, runs gh agentic check, loads standards and the skill catalogue, and handles post-sync actions. Use at every session start before any other skill, and again after a template sync is reported mid-session.
  Triggers: session-start, post-sync

## Operation skills

- **build-catalogue** — Regenerates CATALOGUE.md from every skill's YAML frontmatter in a deterministic, diff-friendly order (grouped by category, alphabetical within category). Use when CATALOGUE.md is missing or stale (any skill mtime newer than the catalogue), or after a skill has been added, removed, or had its frontmatter edited.
  Triggers: on-demand

- **release-notes** — Generates human-readable, well-structured release notes from git commit history and updates the GitHub release body with the AI-written notes. Use when the Release recipe fires on a version tag being pushed to main and the release body needs categorised (Features/Fixes/Documentation/Chores) notes.
  Triggers: on-demand

- **skill-creation** — Produces a correctly-classified, schema-conformant skill file from a human description (reactive mode) or surfaces a proactive suggestion when the agent observes the same substantive action repeated in the current session (proactive mode). Use when the human asks to create a skill that does X, or when the agent notices it has performed three or more substantively-similar actions at a natural pause between user turns.
  Triggers: human-interactive, on-demand

- **update-project-template** — Extracts the live GitHub Project configuration (shortDescription, readme, status field options, and views) and writes it as the canonical .agents/project-template.json so board customisations flow to downstream environments via gh agentic sync. Use when the human asks to save the current project config as the template or to update the project template from the live project (template repo only).
  Triggers: human-interactive

## Information skills

- **notify-user** — Sends an OS-level notification (macOS osascript or Linux notify-send) to alert the human that human action is required or a long-running session has completed. Use when the pipeline reaches a point where a human must act (PR ready for review, fix pushed awaiting workflow restart) or a session has run longer than the configured completion threshold.
  Triggers: on-demand

## Reference skills

- **ask-user** — Canonical harness-neutral interaction shape for every confirmation, classification, disambiguation, or choice prompt raised by any skill — defines when to use a selectable prompt, option constraints, fallback phrasing, and the four canonical prompt shapes (confirm/revise, multi-choice selection, yes/no/later, name-collision). Use inline whenever a skill needs to ask the human for a decision, never as a standalone session.
  Triggers: on-demand

- **capture-design-plan** — Defines the canonical Markdown template for the Design Plan comment feature-design publishes on a Feature issue before any Task sub-issues are created — decomposition rationale, planned tasks, alternatives considered, refactor assessment, and optional codebase findings and risks. Use when feature-design is about to publish its pre-task decomposition rationale as a durable comment on the Feature issue, or when verifying that a published Design Plan comment conforms to the required shape.
  Triggers: on-demand

- **capture-feature** — Defines the canonical markdown body template for every Feature issue created during scoping — user story, context, scope, acceptance criteria in Given/When/Then format, deployment strategy, UX design, notes, and parent link. Use when authoring the body of any new Feature issue, or when reviewing that a Feature issue conforms to the required shape.
  Triggers: on-demand

- **gh-agentic-tool** — Authoritative command reference for the gh agentic CLI extension — every command in the cobra tree with every declared flag, the --raw output contract for agent-oriented data retrieval, and a decision matrix for common agent questions. Use whenever the agent needs to interact with the agentic framework from the command line.
  Triggers: on-demand

- **refactor-assessment** — Canonicalises the search-first, reuse-default, motivate-if-not procedure every code-touching skill performs before writing new code — defines the three permitted outcomes (reuse as-is, reuse via refactor, do not reuse with motivation), the opt-out variant, the single-line recording format, and the loader phrase consumer skills invoke. Use when any skill or agent is about to introduce a new function, type, module, schema, or similar symbol and must first confirm whether existing code already covers the need.
  Triggers: on-demand

- **session-exit** — Defines the canonical universal exit block emitted by every Session and Recovery skill at termination, with the three fixed sections (Produced, Blocked, Next) and worked variants. Use when authoring or updating any session-ending skill, or when verifying that an exit block conforms to the framework shape.
  Triggers: on-demand

- **set-issue-status** — Authoritative pattern for setting a GitHub Project V2 status on an issue via the gh CLI GraphQL API — includes the label-to-status mapping and the exact four-step sequence (resolve node ID, find/create project item, resolve field IDs, update status). Use whenever a pipeline label is applied to an issue (always in the same operation, never as a separate step).
  Triggers: on-demand

- **skill-categories** — Authoritatively defines the six-category skill taxonomy (Session, Recovery, Bootstrap, Operation, Information, Reference) and the YAML frontmatter schema every skill must conform to. Use when authoring a new skill, classifying an existing skill, validating frontmatter, or reasoning about which exit protocol applies to a skill.
  Triggers: on-demand
