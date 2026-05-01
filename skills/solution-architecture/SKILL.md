---
name: solution-architecture
description: Creates or extends the project's Foundation Solution Architecture document (docs/ARCHITECTURE.md) through a conversational, agent-led interview — vision, capability domains, system context, architectural decisions, NFRs, integration points, data model, evolution notes. Operates on the current branch (refuses to run on main); the human pushes and opens a PR manually. Use when the human is starting a new project and needs to author the foundation SA, or wants to extend an existing ARCHITECTURE.md with a new subsystem or decision. Use even when the caller doesn't say "solution-architecture" — phrases like "create the architecture doc", "document the system architecture", "set up the foundation SA", "update ARCHITECTURE.md" should trigger this skill.
triggers: human-interactive
user-invocable: true
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/step-skip-rule.md
  - skills/prompt-user/SKILL.md
  - skills/gh-agentic/SKILL.md
emits-exit-block: true
exit-hands-to: "human: push the branch and open a PR for review; merge to main when satisfied"
---

# Solution Architecture

## Goal

Author or extend `docs/ARCHITECTURE.md` — the Foundation Solution
Architecture document for the project — through an agent-led
interview. The output is a committed update on the human's current
working branch, ready to be pushed and reviewed via a normal PR.

Solution Architecture is **not part of the agentic pipeline** — it
has no GitHub Issue, no label-driven trigger, and produces no PR
through the workflow. It is invoked directly by a human when:

- Starting a new project for which `docs/ARCHITECTURE.md` does not
  exist (mandatory before `requirements-session` can run, since
  that skill enforces the file as a hard precondition).
- Extending an existing project's ARCHITECTURE.md with a new
  subsystem, integration, NFR, or architectural decision.

The skill runs on whatever branch the human is currently on. It
refuses to run on `main` (per RULEBOOK). It commits the file but
does NOT push and does NOT open a PR — the human controls those
steps. This keeps SA review explicitly in-the-loop and avoids
auto-merging foundational decisions.

## Output Artefacts

- A committed update to `docs/ARCHITECTURE.md` on the current
  branch. One commit:
  ```
  docs(architecture): create | extend foundation solution architecture

  <one-line summary of what was created or changed>

  Reuse: opt-out — solution-architecture document is foundational; not derived from existing code
  ```

  The `Reuse:` trailer satisfies the universal reuse-discipline rule
  even though SA work is, by definition, not reusing existing
  symbols.

- The structural conformance of the document: every canonical section
  has at least a placeholder paragraph on first creation; every
  section must trace its content to something the human said
  (anti-fabrication).

No GitHub state mutation. No label transitions. No issues created
or referenced.

The skill's three valid terminal outputs:

**A. Created.** `docs/ARCHITECTURE.md` did not exist on entry; the
skill bootstrapped it via the canonical-sections walk. File
committed.

**B. Extended.** The file existed; the skill loaded it and walked
the human through targeted updates. File committed.

**C. Cancelled.** The human aborted before the commit. No
mutations. The file is unchanged on disk; no commit was made.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy for
  `ON_MAIN_BRANCH`, `USER_CANCELLED`, `GIT_COMMIT_FAILED`,
  `REVISION_LOOP_LIMIT`.
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement
  rule preventing silent skipping. The conditional-step carve-out
  applies to mode-gated steps (Create vs Update) and to optional
  sections the human chooses to skip.

This skill does NOT load `commit-discipline.md` because it does not
produce code changes — there are no tests, no build, no reuse-outcome
to record (the trailer is hand-written per the constant message
above). The discipline applies to code-touching work; the SA
document is human-authored prose.

## Dependencies

- `skills/prompt-user/SKILL.md` — used at every section gate
  (confirm / revise / cancel).
- `skills/gh-agentic/SKILL.md` — used in step 1 to resolve the
  active repo (informational only — surfaced in the banner; no API
  calls).

## Steps

The **step-skip rule** applies. The mode-gated carve-out: Create-only
sections (canonical-walk) are not run when in Update mode, and
vice versa — that's by design, not a violation.

**Resolving the active repo.** Resolve once via:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

and reuse as `<active-repo>` for the banner. The skill makes no
GitHub API calls beyond this.

**Anti-fabrication clause.** Same rule that applies to
`requirements-session` and `requirement-scoping`: every sentence
written into the file must be traceable to something the human
actually said in the conversation, or to verifiable codebase facts
(file paths, library names from `go.mod` / `package.json` / etc).
The agent MAY propose; the human MUST confirm before the proposal
is persisted. Inventing a capability, a constraint, or a decision
the human did not state is forbidden.

**Per-section gate.** Each canonical section has its own
`prompt-user` gate:

```
prompt-user(
  question: "Does this <section name> capture the architecture correctly?",
  header: "Section: <name>",
  options: [
    {label: "Confirm",        description: "Persist this section as drafted."},
    {label: "Revise",         description: "Tell me what to change."},
    {label: "Skip section",   description: "Leave this section as a one-line placeholder; we'll come back to it."},
    {label: "Cancel session", description: "End without writing anything."}
  ]
)
```

- Confirm → persist; continue.
- Revise → ask free-text; cap at 5 revisions; on the 5th raise
  `REVISION_LOOP_LIMIT` (`WARN`) and offer Confirm-as-is or Cancel.
- Skip → render a one-line placeholder ("(to be authored)") in the
  final file; continue. Surface the skipped sections in the exit
  block so the human knows what's still pending.
- Cancel → Output C. Exit.

---

### Section A — Setup

1. **Announce the session.** Print the banner verbatim before any
   tool call:

   ```
   ==========================================================
   === Solution Architecture Session — Started               ===
   ==========================================================
   You are now in Solution Architecture mode. We will create
   or extend docs/ARCHITECTURE.md interactively. The skill
   refuses to run on main; you'll commit directly on your
   current branch and push / PR manually.
   ==========================================================
   ```

   Resolve the active repo (informational) and hold as
   `<active-repo>`.

2. **Branch check.** Apply the refuse-on-main guard per
   `skills/definitions/branch-safety.md`:

   ```bash
   git branch --show-current
   ```

   On `main` / `master` → raise `ON_MAIN_BRANCH` (`ERROR`) using the
   remediation template from the definition with `<suggested-prefix>`
   = `chore/architecture-update`. Exit cleanly. No mutations.

   Otherwise → hold the branch name as `<branch>` and continue.

3. **Detect mode.** Check whether `docs/ARCHITECTURE.md` exists:

   ```bash
   test -f docs/ARCHITECTURE.md && echo exists || echo missing
   ```

   - Missing → `<mode> = create`. Continue to Section B.
   - Exists → `<mode> = update`. Read the file:
     ```bash
     cat docs/ARCHITECTURE.md
     ```
     Hold contents as `<existing>`. Continue to Section C.

---

### Section B — Create mode (only when file is missing)

The agent walks the human through eight canonical sections. Each
gate is a `prompt-user` per the rule in the preamble. Sections
should be drafted concisely — the goal is a *useful* foundation,
not a comprehensive book. A page or two per section is usually
right; the human will extend over time.

4. **Open the conversation. (create only)** Ask the human:

   > "We're starting Solution Architecture for #&lt;active-repo&gt;.
   > In one or two sentences: what is this system, who uses it, and
   > what problem does it solve at the highest level?"

   Wait for the reply. This shapes every subsequent section.

5. **Section 1 — Vision. (create only)** A short paragraph (3–5
   sentences) capturing what the system is, who it serves, and what
   value it delivers. Anchored entirely in what the human said.

   Gate via the per-section prompt-user.

6. **Section 2 — Capability Domains. (create only)** A bulleted
   list of the major capability areas the system covers. Aim for
   3–7 domains; each gets a one-line description. This becomes the
   skeleton against which Requirements are later scoped.

   The agent may propose domains based on the Vision; the human
   confirms or revises.

7. **Section 3 — System Context. (create only)** External actors
   and systems that interact with this one. Format: a textual
   diagram (ASCII / mermaid block) plus a brief description of each
   external relationship. If the human wants a Figma or
   diagrams.net link instead of inline, capture the URL.

8. **Section 4 — Architectural Decisions (ADRs). (create only)**
   The 3–8 architectural decisions that anchor the rest of the
   work. Each ADR:
   ```
   ### ADR-NNN: <decision name>

   **Context:** <why this came up>
   **Decision:** <what was chosen>
   **Consequences:** <trade-offs accepted>
   **Alternatives considered:** <briefly>
   ```

   For first creation, surface the obvious decisions inferred from
   the existing repo (language choice, framework, deployment target,
   data store) and ask the human to confirm/revise/extend. Do NOT
   fabricate alternatives — if the human only ever considered one
   option, say so.

9. **Section 5 — Non-Functional Requirements (NFRs). (create only)**
   Performance targets, availability expectations, security
   constraints, compliance obligations, scale expectations. Bullet
   list with concrete-where-possible numbers. Vague NFRs are worse
   than none — push back on "should be fast" and ask "by what
   measure?"

10. **Section 6 — Integration Points. (create only)** External
    APIs, third-party services, internal services this system calls
    or is called by. For each: protocol, auth model, data direction,
    failure mode. Even if there are zero integrations today, record
    that explicitly — "no external integrations" is itself an
    architectural fact.

11. **Section 7 — Data Model. (create only)** The persistence
    shape (or "stateless" if applicable). For projects with a
    database: list entities, key relationships, ownership, retention.
    For projects without persistence: name what state lives where
    (memory, files, message queues, etc.).

12. **Section 8 — Evolution Notes. (create only)** Where the
    architecture is expected to flex over the next 12 months —
    known unknowns, anticipated scale shifts, integrations on the
    roadmap. This is the "what we'll come back and update" section
    and is the natural target for future Update-mode runs.

13. **Assemble the file. (create only)** Compose the final
    `docs/ARCHITECTURE.md` from the eight confirmed sections plus
    a short header:

    ```markdown
    # Architecture — <project name>

    <Vision section, prose>

    ## Capability Domains

    <bulleted list>

    ## System Context

    <diagram + description>

    ## Architectural Decisions

    <ADR-001, ADR-002, ...>

    ## Non-Functional Requirements

    <bulleted list>

    ## Integration Points

    <table or list>

    ## Data Model

    <text>

    ## Evolution Notes

    <bulleted list>
    ```

    Continue to Section D.

---

### Section C — Update mode (only when file exists)

14. **Render the existing TOC. (update only)** Display the existing
    file's section headings to the human:

    ```
    The current docs/ARCHITECTURE.md has these sections:
      - Vision
      - Capability Domains
      - System Context
      - Architectural Decisions
      - Non-Functional Requirements
      - Integration Points
      - Data Model
      - Evolution Notes
      (or whatever sections actually exist)
    ```

15. **Ask what to update. (update only)** Free-text reply: which
    section(s) are we touching? Or is the human adding a new
    section? Or extending a specific ADR?

    Capture the human's answer as `<targets>` — a list of
    `{ section, intent }` pairs where intent is one of: `add`,
    `revise`, `replace`, `add-ADR`, `add-section`.

16. **Walk the targets. (update only)** For each entry in
    `<targets>`:

    - Locate the existing section in `<existing>` (or the insertion
      point for a new section).
    - Have the conversation about the change.
    - Render the proposed before/after diff (the changed section
      only, not the whole file):
      ```
      Before:
        <current section content>

      After:
        <proposed section content>
      ```
    - Gate via the per-section `prompt-user`.

17. **Apply the changes. (update only)** Compose the updated file
    by splicing the confirmed changes into `<existing>`. Preserve
    everything not touched.

    Continue to Section D.

---

### Section D — Commit and exit

18. **Render the full file to the human.** Display the final
    `docs/ARCHITECTURE.md` content in a fenced markdown block. For
    Update mode, surface the diff hunks alongside the full
    rendering so the human can see what changed at a glance.

19. **Final confirmation.** Last gate before the commit:

    ```
    prompt-user(
      question: "Write and commit docs/ARCHITECTURE.md?",
      header: "Final confirmation",
      options: [
        {label: "Yes, commit",
         description: "Write the file and create one commit on <branch>."},
        {label: "Back to edit",
         description: "I want to revise something — pick a section to revisit."},
        {label: "Cancel",
         description: "Discard the session; nothing written."}
      ]
    )
    ```

    - Yes → continue to step 20.
    - Back → re-enter the appropriate mode (B or C) at a section
      pick. The agent asks which section to revisit.
    - Cancel → Output C. Exit cleanly. No file change.

20. **Write the file.** Use the agent's `Write` tool to write the
    full content to `docs/ARCHITECTURE.md`:

    ```
    Write(path="docs/ARCHITECTURE.md", content=<full-content>)
    ```

    Never use shell `echo` / heredoc — the content may contain
    backticks, dollar signs, and other shell metacharacters that
    would be mangled.

21. **Stage and commit.** One commit, with the prescribed message:

    ```bash
    git add docs/ARCHITECTURE.md
    git commit -m "docs(architecture): <create | extend> foundation solution architecture

    <one-line summary>

    Reuse: opt-out — solution-architecture document is foundational; not derived from existing code"
    ```

    The `<one-line summary>` is generated by the agent from the
    session — for Create: "initial 8-section foundation"; for
    Update: e.g. "add ADR-005 (caching strategy); extend Integration
    Points with Stripe webhook".

    On commit failure → raise `GIT_COMMIT_FAILED` (`ERROR`); the
    file is on disk uncommitted. The human resolves manually.

    Verify the commit:
    ```bash
    git log -1 --format='%s%n%b'
    ```

22. **Emit the exit block.** Match the actual outcome:

    **Output A — Created:**
    ```
    === Solution Architecture Session — Created ===

    Produced:
      - docs/ARCHITECTURE.md (initial 8-section foundation)
      - 1 commit on <branch>: <commit-sha>

    Skipped sections (placeholders only — author when ready):
      - <list, or "none">

    Next: review the file; push the branch (`git push origin <branch>`);
          open a PR for review.
    ```

    **Output B — Extended:**
    ```
    === Solution Architecture Session — Extended ===

    Produced:
      - docs/ARCHITECTURE.md updated (<one-line summary>)
      - 1 commit on <branch>: <commit-sha>

    Sections touched:
      - <list>

    Next: review the diff; push the branch; open a PR.
    ```

    **Output C — Cancelled:**
    ```
    === Solution Architecture Session — Cancelled ===

    Produced: nothing

    docs/ARCHITECTURE.md is unchanged. Re-invoke when ready.
    ```

23. **Terminate the session.** Per `emits-exit-block: true`, invoke
    the host runtime's session-close API if available; otherwise
    halt.

## Error Handling

- `ON_MAIN_BRANCH` from step 2 (current branch is `main` /
  `master`) → severity `ERROR`. Exit cleanly with the remediation
  message; no mutations. Per RULEBOOK, the agent never commits on
  main; this skill enforces that boundary at entry.
- `USER_CANCELLED` (any cancel point — section gate, final
  confirmation) → severity `WARN`. End cleanly. No file changes.
- `GIT_COMMIT_FAILED` from step 21 → severity `ERROR`. The file
  was written to disk but `git commit` failed (hook failure,
  user.email unset, etc.). Surface the underlying error and the
  on-disk path so the human can finish manually:
  ```
  docs/ARCHITECTURE.md was written but the commit failed:
    <git stderr>
  Run `git add docs/ARCHITECTURE.md && git commit` manually
  once the underlying issue is fixed.
  ```
- `REVISION_LOOP_LIMIT` from any section gate (5 revisions
  elapsed) → severity `WARN`; surface current draft, recommend
  Confirm-as-is or Cancel. The human picks; the skill does not
  auto-decide.
- All other errors: propagate (default).
