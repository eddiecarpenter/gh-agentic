---
name: gh-agentic-tool
description: Authoritative command reference for the gh agentic CLI extension — describes every command (info, project check/repair/info/create/join/unlink, mount, auth check/login/refresh, doctor), when to use each, and the agent's decision logic for common situations. Use whenever the agent needs to interact with the agentic framework from the command line or diagnose project health.
category: Reference
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# gh-agentic Tool

## Purpose

Reference skill for the `gh agentic` CLI extension. Describes every available
command, when to use it, and the decision logic the agent should apply. Load
this skill whenever you need to interact with the agentic framework from the
command line.

## When to Invoke

- During `session-init` — to understand the tooling available
- Whenever a command against the agentic framework is needed
- When diagnosing or repairing project health

---

## Command Reference

### `gh agentic info`

Shows the current state of this repo: CLI version, project affiliation, topology,
local framework version, and (for federated repos) the control plane's framework
version.

```bash
gh agentic info
```

**Use this to:**
- Check whether the local framework version matches the control plane
- Confirm project affiliation before starting a session
- Quick health overview without pass/fail detail

**Version sync interpretation:**
- `Framework (local)` matches `Framework (control)` → proceed normally
- Mismatch → run `gh agentic mount` to sync before continuing

---

### `gh agentic project check`

Runs all project health checks and reports pass/fail results:
- `AGENTIC_PROJECT_ID` is set
- Project is accessible (not deleted or inaccessible)
- Topology is resolvable
- Framework is mounted at `.ai/`
- All expected project views are present
- Framework version is in sync with control plane (federated only)

```bash
gh agentic project check
```

**Use this to:**
- Verify the repo is in a good state before starting work
- Identify specific issues to repair

---

### `gh agentic project repair`

Runs all health checks and automatically fixes any repairable issues:
- Framework not mounted → mounts the correct version
- Missing project views → recreates from template
- Framework version out of sync → remounts at control plane version

```bash
gh agentic project repair
```

Issues that cannot be auto-repaired are reported with manual remediation steps.

---

### `gh agentic mount`

Mounts (or remounts) the AI-Native Delivery Framework at `.ai/`.

**Single topology:** mounts at the version recorded in `.ai-version`.
**Federated domain repo:** reads `AGENTIC_FRAMEWORK_VERSION` from the control
plane and mounts that version, updating local `.ai-version` to match.

```bash
gh agentic mount
```

**Use this to:**
- Sync the local framework after the control plane version is updated
- Restore `.ai/` if it has been deleted or corrupted
- First-time mount when joining a project

**Do not** run `mount` with an explicit version on a federated domain repo —
version governance flows through the control plane only.

---

### `gh agentic project info`

Shows detailed project affiliation: project name and ID, topology, control plane
repo, and framework version.

```bash
gh agentic project info
```

---

### `gh agentic project create [title]`

Creates a new GitHub ProjectV2 and sets this repo up as a **federated control
plane**. Only valid on a clean repo (not already affiliated with a project).

```bash
# Non-interactive
gh agentic project create "My Agentic Project"
gh agentic project create "My Agentic Project" --version v2.1.0

# Interactive form
gh agentic project create --interactive
```

Sets `AGENTIC_TOPOLOGY=federated` on the control plane repo.

---

### `gh agentic project join`

Affiliates this repo with an existing project. For already-initialised repos
that need to move between projects. Blocked on uninitialised repos — run
`project init` first.

```bash
# List available projects
gh agentic project join --list

# Interactive picker
gh agentic project join --interactive

# Direct by project name
gh agentic project join "My Agentic Project"
```

---

### `gh agentic project unlink`

Removes this repo's `AGENTIC_PROJECT_ID` affiliation. The GitHub Project itself
is not deleted. Blocked if this repo is the control plane and `docs/` has content.

```bash
gh agentic project unlink
gh agentic project unlink --yes   # skip confirmation (scripts)
```

---

### `gh agentic auth check`

Verifies local Claude credentials and the `CLAUDE_CREDENTIALS_JSON` repo secret
are present and in sync.

```bash
gh agentic auth check
```

Blocked on control plane repos — credentials live on domain repos only.

---

### `gh agentic auth login`

Forces a new Claude Code login and uploads refreshed credentials to the repo secret.

```bash
gh agentic auth login
```

---

### `gh agentic auth refresh`

Uploads current local credentials to the repo secret without triggering a new login.

```bash
gh agentic auth refresh
```

---

### `gh agentic doctor`

Runs a full environment health check: git state, GitHub auth, repo variables,
framework mount, and project affiliation.

```bash
gh agentic doctor
```

---

## Decision Logic

### At session start (`session-init`)

```
gh agentic project check
→ all pass?  proceed
→ any fail?  gh agentic project repair
             re-run check — if still failing, alert human and stop
```

### At requirements / scoping session start

```
gh agentic info
→ versions in sync?  proceed
→ version mismatch?  gh agentic mount
                     confirm sync, then proceed
```

### Framework version out of sync (federated domain repo)

```
gh agentic mount          # reads control plane version, remounts
gh agentic info           # confirm versions now match
```

### Project appears deleted

```
gh agentic project check  # confirms "project not found"
→ alert human: project may have been deleted
→ human runs: gh agentic project unlink
              gh agentic project join "New Project"
```

---

## Rules for the Agent

- **Never run `mount <version>` on a federated domain repo** — version is governed
  by the control plane. Run `mount` with no argument to sync.
- **Never run auth commands on the control plane repo** — they are blocked by design.
- **`project repair` is safe to run automatically** — it is non-destructive and
  idempotent. Run it whenever `project check` reports failures.
- **Always re-run `project check` after `project repair`** — confirm the repair
  succeeded before proceeding.
- **Do not run `project join` on an uninitialised repo** — run `project init` first
  (coming in a future release).
