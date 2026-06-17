# Migration — federation to the control-plane-centralized model

**Status:** Breaking change. Required action for any federation created under the
pre-pivot model (cross-repo sub-issues, domain repos mounting the framework)
before its control plane upgrades to the framework version that ships
requirement [#870] (Features #871/#872/#874/#875, doc alignment #876).

This runbook migrates an existing federation onto the control-plane-centralized
(CP-centralized) model. It is **CP-side**: every step is run on the control plane
or against the GitHub Project, never inside a domain repo's own pipeline (domain
repos no longer have one).

> **No-op for new federations.** A federation created on the current framework is
> already CP-centralized — `gh agentic init` → federated scaffolds the
> domain-grouped `FEDERATION.yaml` and federated-tier docs, `gh agentic project join`
> registers pure-code domain repos, and features are created on the control plane
> from the start. There is nothing to migrate. See *No-op path* below.

## What changed

| Aspect | Pre-pivot model | CP-centralized model (#870) |
| --- | --- | --- |
| Where the pipeline runs | In each domain repo (wrapper workflow) | On the **control plane** only |
| Framework mount | Every domain repo mounted `.agents/` | **Control plane only**; domain repos are pure code |
| Where features live | In their target domain repo | On the **control plane** |
| Feature → requirement link | **Cross-repo** sub-issues (#825) | **Same-repo** sub-issues on the CP (#872) |
| Which repo a feature targets | Implied by the repo it lives in | A **"Target repo"** ProjectV2 field (#872) |
| `FEDERATION.yaml` schema | Flat `repos:` | Domain-grouped `domains:` (#871) |
| Domain registration | `project join` run in the domain repo, with a mount | `gh agentic project join <owner/repo> --domain <name>` run on the **CP**, no mount (#874) |

The headline win: only the control plane carries anything agentic. Federation
maintenance collapses from N repos to one, and domain repos cannot drift.

## Before you start — pre-flight checklist

Run these on the control plane and confirm each before making changes:

- [ ] **You are on the control plane.** `gh agentic info` reports Federation
      topology and a local federation manifest exists at the repo root —
      `FEDERATION.yaml` (canonical) or the legacy `FEDERATION.md`.
- [ ] **The manifest file is named `FEDERATION.yaml`.** The content was always
      YAML; the `.yaml` extension makes that honest and lets editors/tooling treat
      it as YAML. The framework still **reads** a legacy `FEDERATION.md` for
      backward compatibility and migrates it to `FEDERATION.yaml` on the next
      write (e.g. `gh agentic project join`). To migrate eagerly, just rename it:
      `git mv FEDERATION.md FEDERATION.yaml`.
- [ ] **The manifest is on the domain-grouped schema.** It uses `domains:` (not
      the flat `repos:`). If it still uses `repos:`, convert it first — group the
      existing repos under one or more `domains:` entries, each with a `name` and
      `purpose`. `gh agentic check` validates the result.
- [ ] **The manifest is a *pure YAML* file — not a markdown document.**
      `project.ReadFederation` (`internal/project/federation.go`) runs
      `yaml.Unmarshal` over the **entire file**, so it may contain *only* YAML.
      A markdown manifest — architecture prose plus a fenced ` ```yaml ` block —
      fails: the prose lines parse as YAML scalars and `gh agentic info` / `check`
      abort with `cannot unmarshal !!str into project.Federation`. Remove all
      prose and code-fence markers, leaving the bare `domains:` mapping. Move any
      architecture narrative to `docs/SYSTEM_ARCHITECTURE.md`, or keep short notes
      inline as YAML `#` comments.
- [ ] **The control plane is at (or ahead of) the version that ships #870.** The
      "Target repo" field machinery (#872) and CP-side `join` (#874) must be
      available. `gh agentic check` confirms the field exists; `gh agentic repair`
      provisions it (`target-repo-field`) if missing.
- [ ] **You have an inventory of open features.** List every open feature issue
      currently living in a domain repo — these are what you will relocate. If
      there are none, take the *No-op path* below.

## No-op path — a federation with no features yet

If no features have been created in any domain repo (a federation that scoped
requirements but never started feature work, or a brand-new federation), there is
**nothing to relocate**. Complete only these steps:

1. Confirm `FEDERATION.yaml` is domain-grouped and `gh agentic check` is clean.
2. Run the **stale-`.agents` cleanup** (below) for each domain repo, in case any
   carried a mount under the old model.
3. Done. New features will be created on the control plane with the "Target repo"
   field set, per `requirement-scoping`.

No feature work exists, so no feature work can be lost.

## Migration — relocating existing features

> **Guiding rule: no feature work is lost.** Nothing in this section deletes a
> feature's content. Each domain-repo feature is **re-created on the control
> plane** with its full body, labels, design rationale, and task sub-issues
> preserved, and the original is closed with a forwarding pointer only after the
> CP copy is verified complete. Do the relocation one feature at a time and verify
> each before moving on.

For each open feature living in a domain repo:

1. **Re-create the feature on the control plane.** Create a new issue on the CP
   repo carrying the same title, body, and `feature` label. Preserve any in-flight
   state labels (`designed`, `in-development`, etc.) only once the work actually
   continues on the CP — a freshly relocated feature with no CP branch should sit
   at its pre-design state.

2. **Set its "Target repo" field.** On the GitHub Project, set the new feature's
   **"Target repo"** field to the bare name of the domain repo it targets (the
   owner is always the control-plane owner). This replaces the old "the repo it
   lives in is the target" convention. `gh agentic feature target <N>` reports the
   resolved target.

3. **Wire it as a same-repo sub-issue.** Link the new feature as a sub-issue of its
   parent requirement **on the control plane** (same-repo sub-issue). This reverses
   the cross-repo sub-issue link from #825.

4. **Carry over design artefacts.** If the domain-repo feature already had a design
   rationale comment (`<!-- design-plan:v1 -->`) and task sub-issues, re-create
   them under the CP feature so the dev-session can find them. If the feature was
   mid-implementation, move (or re-open) its branch work onto the CP-driven flow —
   the pipeline clones the target repo into `./project` and commits there.

5. **Verify, then close the original.** Confirm the CP feature has the body, the
   "Target repo" field, the same-repo sub-issue link, and any design artefacts.
   Only then close the original domain-repo feature with a comment pointing at the
   CP issue (e.g. `Relocated to <cp-owner/cp-repo>#<N> under the CP-centralized
   model — see docs/migration-cp-centralized.md`). Closing — not deleting —
   preserves the original's history and discussion.

6. **Disable the old cross-repo mechanism.** Once every feature is relocated, the
   domain repos must no longer originate feature work:
   - Remove each domain repo's pipeline wrapper workflow
     (`.github/workflows/agentic-pipeline.yml`) so no pipeline fires there.
   - Confirm requirement-scoping now creates features on the CP with the "Target
     repo" field (it does, per the current `requirement-scoping` skill) — no new
     cross-repo sub-issues will be created.

After this section, every feature lives on the control plane, targets its domain
repo by field, and links same-repo to its requirement. No feature content was
deleted.

## Stale-`.agents` cleanup (per domain repo)

Under the old model each domain repo mounted the framework at `.agents/`. Under
the CP-centralized model domain repos are **pure code** — the mount is stale and
must be removed. For **each repo listed in the control plane's `FEDERATION.yaml`**:

1. **Confirm it is a manifest domain repo**, not the control plane itself. The
   control plane keeps its `.agents/` mount; only domain repos shed it.
2. **Remove the mount and its scaffolding** from the domain repo:
   - Delete the `.agents/` directory (whether a tracked submodule or a legacy
     gitignored clone). For a submodule, deinit and remove it; for a legacy clone,
     delete the directory and strip its `.gitignore` entry.
   - Remove the generated agent entry files that referenced the mount
     (`CLAUDE.md`, `AGENTS.md`) and the pipeline wrapper workflow, if still present
     from the *Disable the old cross-repo mechanism* step.
   - Keep the repo's `AGENTIC_PROJECT_ID` variable — it is how the federation and
     the pipeline still identify the repo as a registered member.
3. **Verify the repo is now pure code.** Running `gh agentic info` inside the
   domain repo reports it as *not under agentic control* — which is correct: the
   control plane is under control, the domain repo is just code. Running
   `gh agentic check` on the control plane confirms the repo is still a linked
   Project member and present in `FEDERATION.yaml`.

This is the re-scoped [#874] AC-3 (stale-`.agents` cleanup), homed here because it
is a CP-side migration step performed where `FEDERATION.yaml` is local.

## After migration

- `FEDERATION.yaml` is domain-grouped, and every registered domain repo is pure code.
- Every feature lives on the control plane with its "Target repo" field set and a
  same-repo sub-issue link to its requirement.
- The pipeline runs only on the control plane; domain repos have no workflow.
- `gh agentic check` on the control plane is clean.

[#870]: https://github.com/eddiecarpenter/gh-agentic/issues/870
[#874]: https://github.com/eddiecarpenter/gh-agentic/issues/874
