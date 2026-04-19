# Refactor Audit — Feature #571 (Centralised Project Context)

**Status:** published (task #579) — design artefact referenced by tasks #580–#587.

## Purpose

Feature #571 consolidates every read of `AGENTIC_*` repo variables and the
`.ai-version` file behind a single canonical resolver in `internal/project/`.
This document is the authoritative **inventory** of every call site that
subsequent migration tasks will rewrite.

It lists:

- every direct `AGENTIC_PROJECT_ID` / `AGENTIC_TOPOLOGY` / `AGENTIC_FRAMEWORK_VERSION`
  read outside `internal/project/`
- every `.ai-version` read/write across Go, shell, workflows, templates, skills,
  and documentation
- every local `AGENTIC_TOPOLOGY` gate / stopgap reference
- the **target resolver field** that will replace each call site after the
  `project.Resolve` API lands in task #580

No code is changed in this task — the rest of the feature uses this document
to size the resolver API (#580), migrate call sites in order (#581–#583),
enforce the boundary (#584), remove the artefacts (#585), and sweep the docs
(#586).

## Scope Boundary

`internal/project/` and `internal/mount/` are **inside** the canonical
boundary:

- `internal/project/` owns the resolver and every `ProjectVarName` /
  `TopologyVarName` / `FrameworkVersionVarName` constant read.
- `internal/mount/` owns the `.ai-version` file I/O (`ReadAIVersion`,
  `WriteAIVersion`, `ReadAIVersionFromGit`). After #585 the file itself goes
  away; these helpers will be removed or repointed at the resolver, but the
  I/O-to-disk boundary stays inside `internal/mount/`.

Everything else — `internal/cli/`, `internal/doctor/`, `internal/init/`,
`internal/auth/`, `internal/projectstatus/`, workflows, templates, skills,
docs — must migrate.

## Proposed Resolver API (sized by this audit)

The resolver sized by this inventory will expose a single
`project.Context` struct with at minimum:

| Field | Source today | Consumers today |
|---|---|---|
| `RepoFullName` (owner/name) | `repository.Current()` | every command |
| `Owner`, `RepoName`, `OwnerType` | `repository.Current()` + `auth.DetectOwnerType` | check, repair, auth, init |
| `Root` | `os.Getwd()` | mount, info, doctor |
| `ProjectID` | `AGENTIC_PROJECT_ID` | check, repair, status_*, auth, info, init, mount, project.*, pipeline_cmd, doctor |
| `ProjectName`, `ProjectDeleted` | `FetchProjectTitle(ProjectID)` | info, check |
| `Topology` (`single` / `federated-cp` / `federated-domain`) | `ResolveTopology` | check, repair, mount, auth, doctor |
| `LinkedRepos`, `ControlPlane` | `FetchLinkedRepos(ProjectID)` | info, mount, auth, check |
| `FrameworkVersion` (local) | `.ai-version` on disk (`ReadAIVersionFromGit` / `ReadAIVersion`) | info, doctor, mount, check |
| `ControlPlaneFrameworkVersion` | `AGENTIC_FRAMEWORK_VERSION` on the CP repo | info, check, mount |
| `VersionInSync` (derived) | compare local vs CP | info, check |
| `IsFederatedControlPlane()` / `IsFederatedDomain()` (methods) | derived from Topology | auth, mount |

The existing `project.ResolveState` (in `internal/project/info.go`) is the
starting point — it already resolves most of the above. Task #580 will
refactor it into `project.Resolve` / `project.Context` and add the remaining
fields.

---

## Section A — Direct reads of `AGENTIC_PROJECT_ID`

All of these must route through the resolver's `Context.ProjectID`.

| File:Line | Function | What it does today | Target resolver field |
|---|---|---|---|
| `internal/cli/check.go:117` | RunE of `newCheckCmdWithDeps` | `runGetVariable(deps.run, info.FullName, project.ProjectVarName)` — pipeline-topology feed | `ctx.ProjectID` |
| `internal/cli/repair.go:219` | `buildPipelineCheckDeps` | `runGetVariable(run, repoFullName, project.ProjectVarName)` | `ctx.ProjectID` |
| `internal/cli/status_requirements.go:78` | `defaultResolveProjectID` | `project.DefaultGetRepoVariable(parts[0], parts[1], project.ProjectVarName)` | `ctx.ProjectID` |
| `internal/cli/status_requirements.go:119` | `runStatusRequirements` | error-message format referencing `project.ProjectVarName` | `ctx.ProjectID` (empty → `projectstatus.ErrProjectNotConfigured`) |
| `internal/cli/status_requirement.go:41` | `runStatusRequirement` | error-message format referencing literal `"AGENTIC_PROJECT_ID"` | `ctx.ProjectID` |
| `internal/cli/status_feature.go:28` | `runStatusFeature` | error-message format referencing literal `"AGENTIC_PROJECT_ID"` | `ctx.ProjectID` |
| `internal/cli/status_features.go:41` | `runStatusFeatures` | error-message format referencing `project.ProjectVarName` | `ctx.ProjectID` |
| `internal/cli/pipeline_cmd.go:162` | `runPipeline` | error-message format referencing `project.ProjectVarName` | `ctx.ProjectID` |
| `internal/cli/auth.go:54` | `isFederatedControlPlane` | `deps.GetRepoVariable(..., project.ProjectVarName)` | `ctx.ProjectID` (guard: empty → bail) |
| `internal/cli/init.go:72` | `newInitCmd` RunE | `deps.GetRepoVariable(..., project.ProjectVarName)` — "already initialised?" guard | `ctx.ProjectID` (empty → allow init) |
| `internal/cli/mount.go:179` | `detectFederatedCP` | `project.DefaultGetRepoVariable(..., project.ProjectVarName)` | `ctx.ProjectID` |
| `internal/cli/project.go:280` | `newProjectJoinCmd` RunE | marks "current" affiliation in picker | `ctx.ProjectID` |
| `internal/cli/project.go:344` | `printAvailableProjects` | marks "current" affiliation in list | `ctx.ProjectID` |
| `internal/cli/project.go:462` | `newProjectSwitchCmd` RunE | marks "current" affiliation in picker | `ctx.ProjectID` |
| `internal/doctor/checks.go:362` | `checkProjectAffiliation` | `checkVariable(deps, "AGENTIC_PROJECT_ID")` under federated-domain | `ctx.ProjectID` presence check (doctor already receives `ProjectID` via `CheckDeps` — migrate the literal-string health-check to use `deps.ProjectID`) |
| `internal/doctor/checks.go:389` | `checkProjectReachability` | `deps.Run("gh", "variable", "get", "AGENTIC_PROJECT_ID", ...)` fallback when `deps.ProjectID` unset | `ctx.ProjectID` (remove the in-doctor fallback once the caller always fills `ProjectID`) |
| `internal/doctor/checks_test.go:692` | test harness | drives `checkVariable(deps, "AGENTIC_PROJECT_ID")` | update alongside doctor migration |
| `internal/doctor/repair_test.go:168,189` | test harness | iterates `AGENTIC_PROJECT_ID` alongside topology vars | update alongside doctor migration |
| `internal/projectstatus/errors.go:13–15` | `ErrProjectNotConfigured` sentinel | error text literally mentions `AGENTIC_PROJECT_ID` | retain the user-facing string (UX) — but source data path must be `ctx.ProjectID` |
| `internal/cli/status_errors.go:111–117` | `renderProjectNotConfigured` | prints the "set AGENTIC_PROJECT_ID" block | retain user-facing message verbatim — no read logic |
| `internal/cli/status_errors_test.go:47,155` | tests of the error block | asserts literal string | update only if message text changes |
| `internal/cli/status_requirements_test.go:250` | comment only | — | — |
| `internal/cli/project_test.go:30` | fake for `GetRepoVariable` dispatch | case `project.ProjectVarName` | will be updated when `project.Deps` wiring changes |

**Identity-name writes** — these stay inside `internal/project/` (or its
adjacent init/wizard packages) which are the authorised writers:

| File:Line | Function | What it does today |
|---|---|---|
| `internal/init/wizard.go:196–200` | `ConfigureRepo` | `gh variable set AGENTIC_PROJECT_ID …` via `scope.ScopeFor` |
| `internal/project/init.go` | `InitRepo` | sets `AGENTIC_PROJECT_ID` via `deps.SetRepoVariable` |
| `internal/project/join.go` | `Join` / `JoinConfirmed` | sets `AGENTIC_PROJECT_ID` |
| `internal/project/switch.go` | `SwitchProject` | sets `AGENTIC_PROJECT_ID` |
| `internal/project/guards.go:97,152,182` | identity guards | read `AGENTIC_PROJECT_ID` via `deps.GetRepoVariable` |

`internal/project/` is the boundary-authorised writer; no migration needed.
`internal/init/wizard.go` is **outside** the boundary — task #583 will route
it through the resolver (or move the write into `internal/project/`).

---

## Section B — Direct reads of `AGENTIC_TOPOLOGY`

All of these must route through `ctx.Topology`.

| File:Line | Function | What it does today | Target resolver field |
|---|---|---|---|
| `internal/cli/mount.go:174` | `detectFederatedCP` | `project.DefaultGetRepoVariable(..., project.TopologyVarName)` — gate on `"federated"` | `ctx.Topology` — check `IsFederatedDomain()` / `IsFederatedControlPlane()` |
| `internal/cli/auth.go:58` | `isFederatedControlPlane` | same pattern — gate on `"federated"` | `ctx.IsFederatedControlPlane()` |
| `internal/cli/init.go:124` | `newInitCmd` RunE | `deps.SetRepoVariable(..., project.TopologyVarName, "single")` — write-path | identity write — keep in `internal/init/` or move to `internal/project/` (task #583) |
| `internal/cli/repair.go:67` | RunE | user-facing prompt string mentions `AGENTIC_TOPOLOGY` | cosmetic — no read; leave |
| `internal/doctor/checks.go:324` | `checkVariablesAndSecrets` | iterates `{"AGENTIC_TOPOLOGY","AGENTIC_FRAMEWORK_VERSION"}` as names to check under federated-CP | `ctx.Topology` / `ctx.ControlPlaneFrameworkVersion` presence — refactor `checkVariable` call to read from resolver |
| `internal/doctor/checks.go:365` | `checkProjectAffiliation` | `checkVariable(deps, "AGENTIC_TOPOLOGY")` under federated-domain | `ctx.Topology` presence check |
| `internal/doctor/repair_test.go:168,189` | tests | iterates literal var names | update alongside doctor migration |
| `internal/scope/scope.go:45`, `scope.go:86–87` | `identityNames` map, doc comment | keeps `AGENTIC_TOPOLOGY` in the identity-name set — used by `scope.ScopeFor` | **keep** — scope.go is a contract for `gh variable set` routing, not a topology reader |
| `internal/scope/scope_test.go:27–28,128` | tests of `identityNames` | — | — |

**Topology writes** — authorised:

| File:Line | What it does today |
|---|---|
| `internal/project/repair.go:247` | `repairTopologyVars` — `SetRepoVariable(..., TopologyVarName, correctTopo)` |
| `internal/cli/init.go:124` | writes `"single"` after single-topology init — move into `internal/project/` (see #583) |
| `internal/init/wizard.go` | may write topology via `scope.ScopeFor` |

The **local `AGENTIC_TOPOLOGY` stopgap** referenced in the feature (the
`federated-domain` fallback that #569 worked around) is **not a separate
code path** — it is the `AGENTIC_TOPOLOGY=federated` variable that
charging-domain had to set manually because `mount.go:174` needed it.
Eliminating the stopgap = eliminating the direct reads at `mount.go:174`
and `auth.go:58` in favour of the resolver (which already falls back to the
linked-repos signal per `topology.go:88–107`). Task #585 lists the removal;
task #581 performs the mount migration that makes it possible.

---

## Section C — Direct reads of `AGENTIC_FRAMEWORK_VERSION`

All of these must route through `ctx.ControlPlaneFrameworkVersion` (read)
or through `project.SwitchVersion` (write).

| File:Line | Function | What it does today | Target resolver field |
|---|---|---|---|
| `internal/cli/mount.go:149` | `resolveMountVersion` | `project.DefaultGetRepoVariable(parts[0], parts[1], project.FrameworkVersionVarName)` on the control-plane repo | `ctx.ControlPlaneFrameworkVersion` |
| `internal/cli/upgrade.go:23` | docstring only | no code read | — |
| `internal/doctor/checks.go:324` | `checkVariablesAndSecrets` | iterates `{"AGENTIC_TOPOLOGY","AGENTIC_FRAMEWORK_VERSION"}` under federated-CP | `ctx.ControlPlaneFrameworkVersion` presence |
| `internal/doctor/repair_test.go:168,189` | tests | — | update alongside doctor |
| `internal/scope/scope.go:46`, `scope.go:87` | identity-name set | keep (routing-only) | — |
| `internal/scope/scope_test.go:28,128` | tests | — | — |

**Framework-version writes** — authorised:

| File:Line | What it does today |
|---|---|
| `internal/project/switch.go:75,92` | `SwitchVersion` — writes `AGENTIC_FRAMEWORK_VERSION` via `SetRepoVariable` on federated CP |
| `internal/project/repair.go:265` | `repairTopologyVars` — writes the latest version when missing on federated CP |

Both stay inside `internal/project/`. No migration.

---

## Section D — `.ai-version` read / write sites

The file is removed entirely in task #585. Everything here is either deleted
or migrated to read from `ctx.FrameworkVersion` (populated from the `.ai/`
git-metadata tag resolved by `mount.ReadAIVersionFromGit`, which already is
the authoritative reader — the flat-file fallback `mount.ReadAIVersion` goes
away with the file).

### D.1 — Go source (reads)

| File:Line | Function | Action in #585 |
|---|---|---|
| `internal/cli/info.go:177–180` | `localFrameworkVersion` — tries `ReadAIVersionFromGit` then falls back to `ReadAIVersion` | **delete the flat-file fallback**; keep `ReadAIVersionFromGit` via the resolver's `ctx.FrameworkVersion` |
| `internal/cli/mount.go:84,204` | `newMountCmdWithDeps` RunE + `localVersionFallback` — reads `.ai-version` to compare against target | migrate to `ctx.FrameworkVersion`; delete `localVersionFallback` entirely |
| `internal/cli/mount_test.go:96,98,296` | tests of mount flow | update to drive the resolver |
| `internal/doctor/checks.go:144,161,164,264` | `checkFramework`, `checkWorkflows` — reads `.ai-version` via `ReadAIVersionFromGit` | keep the read-from-git-metadata path; drop the "file present" assertion in `checkFramework` (now derives from `.ai/` presence alone) |
| `internal/doctor/repair.go:293` | workflow-version repair reads `.ai-version` via `ReadAIVersionFromGit` | keep (still reads from git metadata, not the file) |
| `internal/doctor/checks_test.go:41–42,113,167,195,222` | tests writing `.ai-version` to set up fixtures | update to mount `.ai/` metadata instead |
| `internal/init/wizard.go:286–288` | `CheckAIVersionExists` — stats the `.ai-version` file | replace with a resolver-based "already initialised?" check (`ctx.ProjectID != ""` OR `.ai/` mounted) |
| `internal/init/wizard_test.go:517–522` | tests `CheckAIVersionExists` | update alongside wizard |
| `internal/cli/project_test.go:41` | fake `ReadAIVersion` in test deps | update to resolver-based fake |

### D.2 — Go source (writes)

All `WriteAIVersion` call sites are in `internal/mount/` and its tests. After
#585 the file is no longer written:

| File:Line | Function | Action in #585 |
|---|---|---|
| `internal/mount/mount.go:92–95` | `WriteAIVersion` | delete |
| `internal/mount/mount.go:79–90` | `ReadAIVersion` | delete |
| `internal/mount/mount_test.go:172–213` | tests of the above | delete |
| `internal/mount/firsttime.go:29–32` | writes the file on first-time mount | delete |
| `internal/mount/firsttime_test.go:34–36` | tests of first-time mount | update |
| `internal/mount/switch.go:33–36` | writes the file on version switch | delete |
| `internal/mount/switch_test.go:16,36–37,76,92,102,117,127` | tests of version switch | update |
| `internal/doctor/checks_test.go:42,167,195,222` | writes `.ai-version` to set up fixtures | update (use `.ai/` git-metadata fixture) |
| `internal/init/wizard_test.go:520` | writes `.ai-version` in fixture | update |

### D.3 — Templates

| File:Line | What it shows today | Action in #585 / #586 |
|---|---|---|
| `internal/mount/templates/AGENTS.md.tmpl:11,15` | tells humans to run `gh agentic mount $(cat .ai-version)` | rewrite to `gh agentic mount` (argument-less form already works — it resolves the version from `ctx`) |
| `internal/mount/templates.go:23,27` | the inline Go string literal that generates the same template | rewrite alongside the `.tmpl` |
| `internal/mount/templates_test.go:111–113,147` | asserts the template text | update to the new form |

### D.4 — Workflows

| File:Line | What it does today | Action |
|---|---|---|
| `.github/workflows/agentic-pipeline-reusable.yml:4` | comment mentioning "reads .ai-version" | remove the comment |
| `.github/workflows/agentic-pipeline-reusable.yml:31–38` | runtime step that reads `.ai-version` to set `VERSION` output | **replace** — the workflow should read `AGENTIC_FRAMEWORK_VERSION` on the control plane repo (and fall back to the latest release for single topology). The exact mechanism lands in #585. |

### D.5 — Skills

| File:Line | Current text | Action in #586 |
|---|---|---|
| `skills/session-init.md:142` | "Read `.ai-version` at the repository root and note the mounted framework version." | rewrite to read from `gh agentic info` or the resolver |
| `skills/session-init.md:150` | "The new framework version (from `.ai-version`)" | rewrite to "(reported by `gh agentic info`)" |
| `skills/session-init.md:171` | "If `.ai-version` is missing or unreadable …" | delete/rewrite the whole paragraph |
| `skills/gh-agentic-tool.md:90` | "Single topology: mounts at the version recorded in `.ai-version`." | rewrite: "Single topology: mounts at the latest release (or the pinned `AGENTIC_FRAMEWORK_VERSION` on the CP repo once federated)." |
| `skills/gh-agentic-tool.md:91–92` | "Federated domain repo: reads `AGENTIC_FRAMEWORK_VERSION` from the control plane and mounts that version, updating local `.ai-version` to match." | rewrite without the `.ai-version` mention |

### D.6 — Docs and README

| File:Line | Current text | Action in #586 |
|---|---|---|
| `README.md:89` | "gh agentic mount  # remount at current .ai-version" | rewrite comment |
| `README.md:93` | "The pinned version is stored in `.ai-version` (committed to the repo)." | rewrite — pinned version lives in `AGENTIC_FRAMEWORK_VERSION` on the CP repo (or the `.ai/` git metadata for single topology) |
| `docs/ARCHITECTURE.md:82,94,96,97,99,100,111` | `.ai-version` appears seven times in the Mounting section | rewrite the whole section to describe the new flow |
| `docs/PROJECT_BRIEF.md:76` | "generates `CLAUDE.md`, `AGENTS.md`, `LOCALRULES.md`, `.ai-version`" | drop `.ai-version` |
| `docs/status-verification.md:17,73,74` | mentions `AGENTIC_PROJECT_ID` in UX text | keep — user-facing names are stable |

### D.7 — Recovery logs

| File | Action |
|---|---|
| `recovery-logs/recovery-log-555.md`, `recovery-log-418.md`, `recovery-log-492.md` | historic — **leave alone**; they are point-in-time snapshots |

---

## Section E — Local `AGENTIC_TOPOLOGY` stopgap

There is no separate "stopgap" code path. The stopgap was a **manual**
`gh variable set AGENTIC_TOPOLOGY federated` that charging-domain applied
to work around `mount.go:174` reading the variable directly. The variable
exists as a first-class signal (`internal/project/topology.go`) and is the
primary signal in `ResolveTopology`; removing the stopgap means rewriting
the two CLI sites that read it ad-hoc so the resolver's linked-repos
fallback is actually reachable.

**Sites to rewrite (already listed above under Section B):**

- `internal/cli/mount.go:174` (task #581)
- `internal/cli/auth.go:58` (task #583)

Once both go through `ctx.IsFederatedDomain()` / `ctx.IsFederatedControlPlane()`,
domain repos that never set `AGENTIC_TOPOLOGY` will resolve correctly via the
linked-repos signal, and the stopgap becomes unnecessary.

The `AGENTIC_TOPOLOGY` repo variable **itself** stays — it is written by
`project.RepairTopologyVars` and is the authoritative input to
`ResolveTopology`. Only the ad-hoc reads go away.

---

## Section F — `AGENTIC_FRAMEWORK_VERSION` read/write inventory

Writes — all stay inside `internal/project/`:

- `internal/project/switch.go:75` — `SwitchVersion` broadcasts new version on federated CP
- `internal/project/switch.go:92` — same, else-branch
- `internal/project/repair.go:265` — `repairTopologyVars` sets latest version on federated CP when missing

Reads outside `internal/project/`:

- `internal/cli/mount.go:149` — migrated in #581
- `internal/doctor/checks.go:324` — migrated in #582 (alongside other doctor reads)

All other matches are `scope.go` (identity-name routing — keep) and test
fixtures (update alongside their package migration).

---

## Migration Order (referenced by tasks #580–#587)

1. **#580 — Build `project.Resolve` / `project.Context`**
   Consolidate `ResolveState` + `ResolveTopology` into one entry point that
   returns the full `Context`. Existing call sites that already use
   `ResolveState` (info.go) migrate to the new surface first.

2. **#581 — Migrate `mount`**
   Replace `resolveMountVersion`, `detectFederatedCP`, `localVersionFallback`
   with `project.Resolve` + `ctx.ControlPlaneFrameworkVersion` + `ctx.IsFederatedDomain()`.

3. **#582 — Migrate `check` and `repair`**
   Route `buildPipelineCheckDeps` and the in-line pipeline-topology reads
   through the resolver. Push `ProjectID` / `Topology` into `doctor.CheckDeps`
   from the resolver, not from ad-hoc `runGetVariable` calls.

4. **#583 — Migrate `status`, `info`, `auth`, `upgrade`, `init`**
   - `info.go` already calls `project.ResolveState` — swap to `project.Resolve`.
   - `auth.go:isFederatedControlPlane` → `ctx.IsFederatedControlPlane()`.
   - `init.go:72` → `ctx.ProjectID`. `init.go:124` stays a write but moves
     into `internal/project/` or `internal/init/` as appropriate.
   - `status_*.go`, `pipeline_cmd.go` → use `ctx.ProjectID`; keep
     `ErrProjectNotConfigured` behaviour.
   - `upgrade.go` has no direct reads — only a docstring mention.

5. **#584 — Enforce boundary**
   CI check (`go vet`-style or a dedicated test in `cmd/` or a build tag)
   that fails when any file outside `internal/project/` or `internal/mount/`
   references `AGENTIC_PROJECT_ID`, `AGENTIC_TOPOLOGY`, or
   `AGENTIC_FRAMEWORK_VERSION` directly. Land this **before** #585/#586 so
   the final sweeps are protected.

6. **#585 — Remove `.ai-version`**
   Delete `ReadAIVersion` / `WriteAIVersion` and the file write at
   first-time / switch. Update the mount flow and the reusable workflow to
   read from `AGENTIC_FRAMEWORK_VERSION` (CP) or the `.ai/` git metadata
   (any repo).

7. **#586 — Sweep skills, docs, templates, README**
   Rewrite the artefacts listed in Sections D.3–D.6.

8. **#587 — Regression test: charging-domain scenario**
   Automated test that mounts the new binary against a charging-domain
   fixture **without** the `AGENTIC_TOPOLOGY=federated` stopgap and asserts
   `gh agentic check` passes cleanly.

---

## Out of Scope for #571

- Changing the names or semantics of `AGENTIC_PROJECT_ID`,
  `AGENTIC_TOPOLOGY`, `AGENTIC_FRAMEWORK_VERSION` (these remain a repo-variable
  contract).
- `internal/scope/` — it is a routing contract for `gh variable set`, not a
  reader of the variables' values.
- `internal/projectstatus/errors.go:15` — the user-facing error string is
  stable UX.
- Recovery-log history under `recovery-logs/`.
- Domain-repo stopgaps (`AGENTIC_TOPOLOGY=federated` set manually on a
  domain repo) — those disappear naturally once the domain picks up the
  migrated binary; the feature does not reach across repo boundaries.

---

Reuse: n/a — documentation-only inventory for task #579; no new symbols introduced.
