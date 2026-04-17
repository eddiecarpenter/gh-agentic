# Session Init

## Purpose

Load the project environment at the start of a session, or reload it after a
framework mount update. Ensures the agent operates with the correct context,
repo state, rules, and skills before doing any work.

## When to Invoke

- **New session starts** — invoke this skill before anything else
- **Framework updated mid-session** — the human says any of the following:
  - "template synced"
  - "I just synced the template"
  - "framework updated"
  - `/template-synced`

## What the Agent Does

Execute these steps in order — do not skip any:

1. Check whether `POST_SYNC.md` exists in the repository root.
   - If it exists: invoke the `post-sync` skill (from `skills/post-sync.md`).
     - If the post-sync skill **exits** (automated session): session-init also exits
       immediately — do not execute any further steps.
     - If the post-sync skill **completes** (interactive session): continue with the
       remaining steps below.
   - If it does not exist: continue normally — no change in behaviour.

2. Read `docs/PROJECT_BRIEF.md` — understand what the system is and how it works.
   If the file does not exist, note this and continue — do not block.

3. **Run health check and repair.**
   ```bash
   gh agentic check
   ```
   - All checks pass → continue
   - Any check fails → run `gh agentic repair`, then re-run `gh agentic check`
   - Still failing after repair → **stop and alert the human** with the exact failure output.
     Do not proceed until the repo is in a healthy state.

   This check applies to both interactive and automated (CI) sessions — do not skip it.

   If `gh agentic` is not installed, fall back to verifying `AGENTIC_PROJECT_ID` manually:
   ```bash
   gh variable list --json name --jq '.[].name' | grep -q AGENTIC_PROJECT_ID
   ```

4. Check whether `REPOS.md` exists in the repository root.
   - If it does not exist: this is a single-repo topology — skip this step entirely and continue.
   - If it exists: read it. For each repo with status `active`, derive its local directory as
   `<type>s/<name>` (e.g. `type: domain` → `domains/<name>`, `type: tool` → `tools/<name>`).
   For each unique type, ensure the type folder (`<type>s/`) exists — if not:
   a. Create the folder with a `.gitkeep` file
   b. Stage it: `git add <type>s/.gitkeep`
   c. Add `<type>s/*/` to `.gitignore` and stage that too: `git add .gitignore`
   d. Commit both: `chore: bootstrap <type>s/ directory`
   Check whether each `<type>s/<name>` directory exists locally. If any repos are
   missing:

   **Interactive session (GITHUB_ACTIONS is not set):** list the missing repos and
   ask the user whether to clone them before proceeding.
   Clone command: `git clone <repo> <type>s/<name>`
   If the user declines, continue the session but limit all work to repos that are
   present locally. Do not reference, modify, or make assumptions about the content
   of repos that were not cloned.

   **CI session (GITHUB_ACTIONS=true):** note the missing repos in output and
   continue immediately — do not prompt, do not block. Limit work to repos that
   are present in the workspace.

5. Query open Requirement issues in the agentic repo:
   `gh issue list --repo <agentic-repo> --label requirement --state open --json number,title,labels`

6. For domain sessions — query open Feature issues in the domain repo:
   `gh issue list --label feature --state open --json number,title,labels,body`

7. Read the relevant standards file from `standards/` for the domain language
   (e.g. `standards/go.md` for Go domains)

8. Load skills — read every `.md` file in `skills/` (framework-managed) and in
   the repo root `skills/` (local, if it exists outside `.ai/`). Local skills take
   precedence over framework skills of the same name.

   **Automation-only skills** — the following skills are loaded for reference only.
   They must never be executed in an interactive session:
   - `feature-design.md` — runs automatically on `in-design` label
   - `dev-session.md` — runs automatically on `in-development` label
   - `pr-review-session.md` — runs automatically on PR review submission
   - `issue-session.md` — runs automatically on issue assignment

   If asked to run any of these interactively, refuse and explain that GitHub Actions
   handles them automatically.

   **Note:** `dev-session.md` checks for `recovery.md` at startup. If found, it
   enters recovery mode — skipping completed tasks and resuming from where the
   previous session left off. See `dev-session.md` for full details.

9. Read `.ai-version` at the repository root and note the mounted framework version.
   If absent (e.g. in the gh-agentic repo itself), skip and continue.

## On Completion

**New session:** proceed with the work for this session.

**Framework updated mid-session:** confirm to the human before resuming work:
- The new framework version (from `.ai-version`)
- The list of files reloaded (protocol + skills)
- Any skills added or removed compared to what was previously loaded (if detectable)

## Rules

- Do not begin any work until all steps are complete
- Do not modify any files during this skill — steps 1–9 are read-only except for
  the post-sync actions in step 1 (if `POST_SYNC.md` is present) and the type
  folder bootstrap in step 4 (only if a folder is missing)
- If `.ai-version` is missing or unreadable, warn the human and continue —
  the version file is informational, not blocking
- There is no STATUS.md — current state is derived from GitHub Issues
- **Inline status updates**: this skill does not apply pipeline labels. If a future
  change adds a pipeline label transition here, it must include an inline project status
  update following `set-issue-status.md` — hard-fail if `AGENTIC_PROJECT_ID` is not set
