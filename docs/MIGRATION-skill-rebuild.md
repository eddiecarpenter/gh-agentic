# MIGRATION — Skill Rebuild (skills-review branch)

The `skills-review` branch rewrites several core skills under the new
skill-spec. The rewritten skills assume some label/status names and
behaviours that the Go CLI (init / check / repair) does not yet enforce.
Listing the gaps here so they aren't lost.

## Status

Skills updated on `skills-review`:

- `requirements-session` (patched)
- `set-issue-status` (new primitive)
- `requirement-scoping` (renamed from `feature-scoping`, substantively
  rewritten)
- `step-skip-rule` (new definition)

## Renames the CLI must catch up to

The skills now reference these names; the Go side must learn them
before this branch can be deployed in any domain repo:

### Project Status field options

| Old | New |
|---|---|
| `Scheduled` | `Ready to Implement` |

### Issue labels (kebab case)

| Old | New | Notes |
|---|---|---|
| `scheduled` | `ready-to-implement` | Requirement lifecycle label |
| `needs-ux-design` | `needs-interactive-design` | Classification: design must run interactively |
| (none) | `interactive-design` | New trigger label, parallel to `in-design` |
| (none) | `designed` | Feature parked state — design complete, awaiting `trigger-implementation`. Only set when interactive feature-design ends with the "Stop here" choice. Headless and trigger-now paths skip this label. |
| (none) | `design-in-progress` | Concurrency beacon for `feature-design`. Applied at session entry, removed on exit (success, parked, error, or cancel). A second session sees this label and refuses to compete (headless) or warns the human and asks before continuing (interactive). |

### Project Status field options (additions)

| Old | New |
|---|---|
| (none) | `Designed` — paired with the `designed` label above |

### Skill name (path/dir)

| Old | New |
|---|---|
| `feature-scoping` | `requirement-scoping` |

This is a skill rename (directory + frontmatter), not a label/status
concern. Some legacy references survive in `recipes/feature-scoping.yaml`
and `internal/frameworkcheck/frameworkcheck_test.go` — those are tied to
the archived flat-skill layout and need their own cleanup pass.

## Files that need updating in the Go CLI

### Stage / status enum and parser

- `internal/projectstatus/types.go`
  - `StageScheduled = "scheduled"` → `StageReadyToImplement = "ready-to-implement"` (rename const + value)
  - `ParseStage`: match `"scheduled"` → match `"ready-to-implement"` (and `"ready to implement"` once `space → hyphen` normalisation runs)
- `internal/projectstatus/types_test.go` — tests reference the old names

### Pipeline command

- `internal/cli/pipeline.go` — references `StageScheduled` in the stage-order list
- `internal/cli/pipeline_test.go` — test fixtures
- `internal/cli/status.go` — may reference `in-design` (unchanged) but check for related usage
- `internal/cli/status_requirements_test.go` — fixtures

### init / check / repair

The init wizard does NOT currently create the lifecycle labels
(`backlog`, `scoping`, `ready-to-implement`, `in-design`,
`interactive-design`, `in-development`, etc.) on the repo; they're
expected to exist already or to be created manually. Ditto for the
project's Status field options. Worth deciding:

1. **Continue assuming labels/options pre-exist** — document the
   required label set in setup docs; init does not enforce. Cheapest.
2. **Add label/option provisioning to init** — `init` creates the full
   set on first run. `repair` re-applies. More robust but bigger scope.

The `requirement-scoping` skill currently propagates label/option
mismatches as `STATUS_TRANSITION_FAILED` and recommends `gh agentic
repair`. If repair doesn't yet provision labels/options, the message is
misleading. Either fix the message or build the provisioning.

## Suggested sequence for the catch-up work

1. Update `projectstatus/types.go` const + parser (mechanical).
2. Update tests for the const rename.
3. Update `cli/pipeline.go` stage-order list.
4. Decide on init/repair provisioning; if yes, add it.
5. Run the Go test suite; fix anything that fell over.

This work belongs in its own PR or as a follow-up commit chain on
`skills-review`. Not blocking for the skill rewrites themselves, but
**must land before the rewritten skills are exercised against a real
repo** — otherwise `gh agentic status` will report `StageUnknown` for
every Requirement at `ready-to-implement` and the framework's status
queries will silently misbehave.
