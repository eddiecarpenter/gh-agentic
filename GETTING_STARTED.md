# Getting started with gh-agentic

This guide walks you through installing `gh-agentic` and running your first agentic delivery cycle — capturing a Requirement, scoping it into a Feature, and watching the pipeline open a pull request with the implemented work.

The examples below use **Claude Code** as the agentic workbench. The framework is workbench-agnostic — anywhere you can run agent skills (Goose, OpenCode, an MCP-aware IDE, or any other model that supports tool use and skill invocation) the same flow works. Skill invocations are the only workbench-specific bit, and the substitution is mechanical.

---

## Prerequisites

| Requirement | Why |
|---|---|
| [git](https://git-scm.com) | Submodule-based framework mount and pipeline checkouts |
| [GitHub CLI](https://cli.github.com), authenticated (`gh auth login`) | All `gh agentic` commands route through `gh` |
| [Claude Code](https://claude.ai/code) | The agentic workbench used in the examples below; also where credentials live for the headless CI agent |
| A GitHub repository | The agentic environment is per-repo; you can use a fresh repo or an existing one |

Credentials for the headless agent (the one that runs in GitHub Actions) are sourced from your local Claude Code install:

- **macOS:** the macOS Keychain entry `Claude Code-credentials`
- **Linux:** `~/.claude/.credentials.json`

### Recommended: self-hosted runners (ARC)

The agentic pipeline runs on every phase transition — design, dev-session, PR review, release. Each headless agent run typically takes 10–60 minutes, and a single Feature can fire several runs end-to-end. On GitHub-hosted runners this **consumes Actions minutes quickly** and is the dominant cost of running the framework at scale.

The recommended setup for any non-trivial agentic project is **[Actions Runner Controller (ARC)](https://github.com/actions/actions-runner-controller)** — a Kubernetes-based self-hosted runner with horizontal autoscaling. Once ARC is deployed, the cost of an agent run becomes the cost of the underlying compute (which you control) rather than the Actions-minutes meter. ARC also gives you control over the runner image (preinstalled tools, language toolchains, network policy).

**Hard requirement for local models.** If you want to run agent sessions against a local LLM (Ollama, vLLM, llama.cpp, an in-cluster model service), you **must** have a self-hosted runner — GitHub-hosted runners cannot reach into your network to talk to a local model server. ARC + an in-cluster model deployment is the standard topology.

**Configuration knob.** During `gh agentic init` the wizard collects a `RUNNER_LABEL` value (e.g. `ubuntu-latest` for GitHub-hosted, or a custom label like `arc-runners` matching your runner-set). The reusable workflows pin every job to that label — so you can switch your repo from GitHub-hosted to ARC by changing one repo variable, no workflow edit required.

---

## 1. Install the CLI extension

```bash
gh extension install eddiecarpenter/gh-agentic
gh agentic --version
```

To upgrade later:

```bash
gh extension upgrade agentic
```

---

## 2. Bootstrap the agentic environment

In your repo root:

```bash
gh agentic init
```

The interactive wizard:

1. Detects the current repo and your GitHub identity.
2. Asks for the framework version, stack, agent identity, runner label, agent provider/model.
3. Either creates a new GitHub Project or joins an existing one (single-topology by default; federated topology is available for multi-repo projects).
4. Mounts the framework at `.ai/` as a tracked git submodule pinned to the chosen version.
5. Generates `CLAUDE.md` and `AGENTS.md` (the agent entry files).
6. Writes the reusable workflow callers under `.github/workflows/`.
7. Configures GitHub repo secrets and variables: `AGENTIC_PROJECT_ID`, `AGENTIC_FRAMEWORK_VERSION`, `AGENTIC_APP_CLIENT_ID`, `AGENTIC_APP_PRIVATE_KEY`, `PROJECT_PAT`, `CLAUDE_CREDENTIALS_JSON`, etc.

Verify everything is wired up:

```bash
gh agentic check
```

A clean health check looks like:

```
✓ Project reachable
✓ Framework: .ai/ mounted (vX.Y.Z), version pinned, .ai/ not in .gitignore
✓ Workflows: agentic-pipeline.yml @vX.Y.Z, release.yml @vX.Y.Z
✓ Variables & secrets: all configured
```

If any check fails, run `gh agentic repair` — it auto-fixes what it can and prints remediation commands for the rest.

Commit the staged changes (`.gitmodules`, `.ai` gitlink, `CLAUDE.md`, `AGENTS.md`, workflows) and push.

---

## 3. Establish the architectural baseline

Before you can capture your first Requirement, the framework needs an **architectural baseline** to anchor against. This is a non-negotiable gate: the [`requirements-session`](skills/requirements-session/SKILL.md) skill refuses to run if `docs/ARCHITECTURE.md` is missing — without an architectural baseline, Requirements have nothing to anchor against and tend to be vague or contradictory.

Open Claude Code in the repo:

```bash
claude
```

Claude Code reads `CLAUDE.md` at session start (which imports `AGENTS.md`, which imports the framework `RULEBOOK.md` and your `LOCALRULES.md`). The full agentic ruleset and skill catalogue load automatically.

Then invoke:

```
/solution-architecture
```

The agent runs the [`solution-architecture`](skills/solution-architecture/SKILL.md) skill, which leads a conversational interview to capture the project's **Foundation Solution Architecture** — vision, capability domains, system context, architectural decisions, NFRs, integration points, data model, evolution notes — and writes the result to `docs/ARCHITECTURE.md` on the current branch. You commit and push; the resulting PR is reviewed and merged like any other change.

Once `docs/ARCHITECTURE.md` exists on `main`, the requirements gate clears and you can move on to step 4.

> **Looking ahead:** issue [#673](https://github.com/eddiecarpenter/gh-agentic/issues/673) extends this skill to lead a Domain-Driven Design (DDD) interview — bounded contexts, ubiquitous language, context map, aggregates, domain events, subdomain classification — so the architecture doc establishes a shared language that carries through to every downstream phase. For now the interview is unstructured but produces a working baseline.

The same skill is used later to **extend** the architecture when a new subsystem or significant decision lands. It is not a one-shot — it is the canonical edit path for `docs/ARCHITECTURE.md`.

---

## 4. Run your first delivery cycle

The pipeline moves work through five phases. The human gates **entry** into the agent pipeline (applying the trigger label to a Feature) and **exit** (reviewing the PR). Within the agent pipeline, design hands off to implementation autonomously — once headless design completes, the dev-session fires immediately without waiting for human input. This is the only autonomous transition in the pipeline; every other handoff is gated.

```
Requirement → Scoping → Design ─┐
   (human)     (human)   (agent)│ autonomous handoff
                                ▼
                        Implementation → Review → Merged
                            (agent)     (human)  (human)
```

For Features flagged `needs-interactive-design`, the agent ends design with a 3-way prompt instead — *trigger now*, *park at `designed`*, or *cancel* — so you can review the Design Plan in foreground before code is written.

### 4a. Capture a Requirement

In the same Claude Code session:

```
/requirements-session
```

The agent runs the [`requirements-session`](skills/requirements-session/SKILL.md) skill, which interviews you about the underlying business need (role, capability, outcome) and creates a `requirement`-labelled GitHub issue at `backlog`.

Equivalent in another workbench: invoke the same skill (the SKILL.md is workbench-agnostic) or paste its instructions into a system prompt and let your model walk you through the interview. The artefact is the GitHub issue either way.

### 4b. Scope it into Features

```
/requirement-scoping
```

Walks the Requirement through nine artefacts (problem framing, decomposition, MVP, acceptance criteria, deployment strategy, parking lot…) and produces one or more `feature`-labelled GitHub issues, each wired as a sub-issue of the parent Requirement. At the end, you choose which Features to trigger for design.

### 4c. The agent designs and implements

The scoping skill ends by asking which Features to trigger; for any you select, it invokes the [`trigger-design`](skills/trigger-design/SKILL.md) primitive — **don't apply `in-design` or `interactive-design` by hand**. `trigger-design` reads the Feature's labels and picks the right path:

- Feature carries `needs-interactive-design` (UX/UI work, novel architecture, or anything you flagged for foreground review during scoping) → `interactive-design`. You run `/feature-design <N>` in your workbench.
- Otherwise → `in-design`. The Stage 3 workflow picks it up automatically.

For headless design (`in-design`):

1. The **design phase** runs in CI — the agent reads the Feature, drafts a Design Plan rationale (posted as a comment), creates ordered Task sub-issues, and creates the feature branch.
2. When design completes, it **auto-applies `in-development`** via the `trigger-implementation` primitive and the **implementation phase** fires immediately — no second human gate. The agent walks each Task in order, writes code, runs tests, commits with the conventional task-format message, pushes, and opens the pull request.
3. The Feature transitions to `in-review` once the PR is open.

For interactive design (`interactive-design`):

1. You run `/feature-design <N>` in Claude Code (or the equivalent in your workbench). The agent walks the design phase in foreground — exploration, rationale draft + confirm, Task list confirm — and posts/creates the same artefacts as the headless path.
2. At end-of-design the agent presents a 3-way prompt: **trigger now**, **park at `designed`**, or **cancel**. Pick **trigger now** to fire implementation immediately (same as the headless path); pick **park** if you want the Feature to wait.
3. To un-park a Feature later, invoke `/trigger-implementation` — same primitive the headless design auto-fires, just human-driven. **Do not apply `in-development` by hand**; let the primitive do the right label/status moves atomically.

You can watch progress at any time with:

```bash
gh agentic status pipeline
```

…which renders the Requirements and Features side by side as columns of a Kanban board, showing the stage of each.

### 4d. Review the PR

Open the PR. If you request changes, the agent (`pr-review-session`) automatically picks up reviewer comments, addresses them in fresh commits, and pushes back. When you approve and merge, the Feature transitions to `done`.

That's a full cycle. The same pattern holds for every Requirement.

---

## 5. Daily commands

| What you want | Command |
|---|---|
| See where everything is | `gh agentic status pipeline` |
| Detail on one requirement | `gh agentic status requirement <N>` |
| Detail on one feature | `gh agentic status feature <N>` |
| Verify the environment | `gh agentic check` |
| Auto-fix environment issues | `gh agentic repair` |
| Bump the framework version | `gh agentic upgrade <new-version>` |
| Refresh expired Claude credentials | `gh agentic auth refresh` |
| Show repo's agentic state | `gh agentic info` |

Add `--raw` to any `status` subcommand for an agent-oriented payload (TSV / frontmatter+verbatim) suitable for scripting and downstream skills.

---

## 6. Using a different workbench

Anywhere the agent can:

- read framework files at `.ai/` (skills, recipes, standards, rulebook),
- shell out to `gh` and `git`,
- and follow a SKILL.md playbook,

…the same delivery flow runs. Concrete substitutions for the examples above:

| Step in this guide | Claude Code | Goose | Generic agent |
|---|---|---|---|
| Bootstrap architecture | `/solution-architecture` | `goose run --recipe .ai/recipes/solution-architecture.yaml` | Load `skills/solution-architecture/SKILL.md` and follow it |
| Capture a requirement | `/requirements-session` | `goose run --recipe .ai/recipes/requirements-session.yaml` | Load `skills/requirements-session/SKILL.md` and follow it |
| Scope a requirement (calls `trigger-design` for selected Features) | `/requirement-scoping` | analogous recipe | analogous skill load |
| Run interactive design | `/feature-design <N>` | analogous recipe | analogous skill load |
| Un-park a `designed` Feature | `/trigger-implementation` | analogous recipe | analogous skill load |

The headless phases (`in-design` and `in-development` triggered via labels) always run via Goose in CI — that's hard-wired into the reusable workflows. The interactive phases are workbench-of-choice.

---

## 7. Where to next

- [`README.md`](README.md) — what gh-agentic *is* and how the pipeline is structured.
- [`RULEBOOK.md`](RULEBOOK.md) — the universal rules every session follows.
- [`skills/`](skills/) — per-phase playbooks (one directory per skill, each with a SKILL.md).
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — package structure and mount model.
- [`eddiecarpenter/ocs-testbench`](https://github.com/eddiecarpenter/ocs-testbench) — a live downstream repo running this framework end-to-end.
