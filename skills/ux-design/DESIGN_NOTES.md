# UX_DESIGN — Skill Design Notes

Captured from authoring `docs/UX_DESIGN.md` for a real project
(ocs-testbench). These notes are input for refining the `ux-design`
skill — not a SKILL.md draft. They focus on the *general* lessons, not
the project-specific content.

---

## 0. Two artefacts, one filename collision

The existing `skills/ux-design/SKILL.md` produces a **per-feature
handoff** at `docs/design/feature-<N>/UX_DESIGN.md`. What we built in
ocs-testbench is a **project-wide canonical spec** at
`docs/UX_DESIGN.md`. Both end up named `UX_DESIGN.md`; they have
different lifetimes, audiences, and purposes:

| Artefact | Lifetime | Authored when | Consumed by |
|---|---|---|---|
| Project-wide `docs/UX_DESIGN.md` | Long-lived; evolves with the codebase | Once at project birth, then maintained | Every UI feature, every PR review, the per-feature UX skill |
| Per-feature `docs/design/feature-<N>/UX_DESIGN.md` | Bound to one feature | Once per UI feature, before dev-session | The dev-session implementing that feature |

**Recommendation**: split the skill into two, or keep one skill with a
mode flag. Either way, the *project-wide* artefact is missing from the
framework and needs explicit support. The two interact: the per-feature
artefact should reference the project-wide one ("button styles per
`R-BTN-DESTRUCTIVE-01`"), not re-derive rules from scratch.

The notes below address the **project-wide** skill.

---

## 1. The skill governs three artefacts, not one

| Artefact | Owner | Lifetime |
|---|---|---|
| Project-wide `docs/UX_DESIGN.md` | Project | Per-repo, evolves with the codebase |
| Best-practice baseline | Framework | Shipped with the skill, inherited by every project |
| Per-feature design output (Figma refs, screen specs) | Project (per feature) | Created in feature-design phase, references UX_DESIGN.md |

The most under-appreciated of these is the baseline. Without it, every
project re-derives WCAG, validation timing, modal dismissal rules, focus
management, etc., and gets it inconsistently right.

**Recommendation**: ship the skill with a `baseline.md` containing the
universal best-practice rules. The project's `UX_DESIGN.md` declares
overrides + project specifics (component library, palette, layout
decisions). When the skill loads context for a UI task, it loads
*baseline + project doc + relevant exceptions*.

---

## 2. The artefact has two audiences, with the stricter one as design driver

Humans tolerate prose, infer intent, ask for clarification. AI agents
can't. Designing the file for the AI consumer makes it more useful for
humans too. Practical implications, all of which we adopted in the real
file:

- **Stable rule IDs** (`R-<AREA>-<TOPIC>-<NN>`) so the skill can cite
  them in PR review and the user can grep for them.
- **MUST / SHOULD / MAY** semantics (RFC 2119) — no ambiguity about
  which rules are negotiable.
- **Token references, not values** — `var(--mantine-color-brand-light)`
  not `#cddeff`. Otherwise the doc drifts from the theme.
- **Anti-examples** — every rule benefits from a "do this / not that"
  pair. The skill can match the wrong shape, not just describe the
  right one.
- **Predictable section anchors** — the skill lifts sections into
  prompts; renaming `## Buttons` to `## Button styling` breaks every
  prompt that referenced the old anchor. Encourage `## §N.M Title`
  format with stable numbering.

---

## 3. Exceptions are load-bearing — design them first-class

Without a sanctioned exception mechanism, the skill flags legitimate
code as non-compliant, the team learns to ignore it, and the skill
becomes noise.

The schema we used and recommend codifying:

- **Rule deviated from** (rule ID).
- **Where** (specific files / components).
- **Deviation** (what is done differently).
- **Reason** (why this is correct here).
- **Generalisation** (how to recognise other valid candidates).

The *generalisation* line is what makes the exception extensible.
Without it, every new instance of the same valid pattern requires a
separate exception entry, and the list grows monotonically.

**Recommendation**: codify this schema in the framework. The skill
should refuse an exception entry that's missing any of the five fields
— particularly *reason* and *generalisation*.

Real example from ocs-testbench: peer-status toasts auto-close even at
`error` severity, because they're ambient system-state notifications
with a visible source-of-truth (the row badge), not consequences of a
user action. The generalisation rule captures the principle, not just
this one case.

---

## 4. The bootstrap procedure matters, and isn't obvious

Writing a project-wide UX spec from scratch *without* an existing
codebase is one problem (greenfield). Writing it for an existing
codebase that has de-facto patterns — what most projects face — is a
different problem. The procedure that worked:

1. **Audit the existing code** — inventory the de-facto patterns. Don't
   skip this; you can't write canonical rules without knowing what's
   actually being done.
2. **Identify conflicts** — places where the same UX concern has
   multiple solutions in the codebase.
3. **Score each conflict against best practice** — the loudest, most
   common, or most recent pattern is *not* automatically the canonical
   one. Modern best practice wins; current code becomes the bug list.
4. **Extract canonical rules** — write them prescriptively
   (best-practice-first), independent of which version the code currently
   matches.
5. **Capture intentional deviations** as exceptions (§Exceptions).
6. **File the unintentional deviations** as a separate Requirement
   issue to be fixed via the normal pipeline. **They do not belong in
   UX_DESIGN.md** — the doc is forward-looking; the bug list is a
   transient artefact.

The fifth and sixth steps are easy to get wrong. A common failure mode
would be enshrining current-code-as-canonical because it's easier than
fighting for best practice. The skill should structurally separate
*"what the rule says"* from *"what the code does"* and never let the
latter dictate the former.

**Recommendation**: the skill ships with a bootstrap recipe that walks
through these six steps. The output is two artefacts: `docs/UX_DESIGN.md`
and a GitHub Requirement issue. The skill explicitly closes the loop
between them by linking from the Requirement issue back to specific
rule IDs.

---

## 5. The decision axes, not the verdicts, are the framework's value

We hit several "this vs. that" decisions: validation on-change vs.
on-blur, modal outside-click yes/no, toast position top vs. bottom,
required-asterisk vs. optional-suffix, click-to-edit rows yes/no.

A naive skill picks one verdict per axis and prescribes it everywhere.
That's wrong — different projects have different constraints (desktop
operator tool vs. consumer mobile vs. data-dense ops console). What the
skill *should* do is enumerate the **decision axes** and require the
project to declare its position on each, with the framework offering a
recommended default backed by best-practice citation.

Suggested axes (non-exhaustive, drawn from the ocs-testbench writing):

- **Validation timing**: on-change / on-blur / on-submit / hybrid.
  *Recommended default: on-blur, plus revalidate-on-change after first
  failed submit.*
- **Modal dismissal**: outside-click closes / Esc only / explicit only.
  *Recommended default: destructive confirms = explicit only.*
- **Required indicator policy**: asterisk-required / optional-suffix /
  ratio-based switch. *Recommended default: ratio-based (≥60% required
  → mark optional).*
- **Row click semantics**: navigates / opens edit / does nothing.
  *Recommended default: does nothing — actions go through kebab.*
- **Toast position**: top-right / bottom-right / bottom-center.
  *Recommended default: top-right.*
- **Empty-state richness**: bare-text / illustrated / context-dependent.
  *Recommended default: context-dependent — first-run = illustrated,
  filtered-empty = bare.*
- **Destructive action recovery**: confirm-modal / undo-toast / both.
  *Recommended default: confirm-modal; undo-toast is a project-level
  upgrade.*
- **Toast persistence rule**: severity-based / consequence-based /
  importance-based. *Recommended default: importance-based —
  must-see-vs-safe-to-miss, regardless of severity.*
- **Touch-target floor**: 36×36 / 44×44 / 48×48. *Recommended default:
  36×36 for desktop tools, 44×44 if any touch use is anticipated.*

A project doc that doesn't take a position on each axis is incomplete.
The skill should flag missing axes during a *"validate the
UX_DESIGN.md"* step rather than letting projects ship a half-specified
doc.

---

## 6. Lint-ability is a first-day decision, not a post-hoc retrofit

If rules are stated such that a script can scan the codebase and emit
violations, the skill becomes a continuous-quality tool. If rules are
stated as pure prose, every check requires an LLM call.

Practical rules to enable linting:

- Each rule names the **AST shape** or **regex pattern** that violates
  it where possible. *"Drawer Delete uses `variant="outline"
  color="red"`"* is greppable; *"emphasis matches consequence"* isn't.
- Tokens, not values. Linters can resolve tokens to allowed sets.
- File-path scopes. *"This rule applies to files matching
  `**/*Form.tsx`"* lets the linter narrow the search.

Some rules will always require LLM-grade judgement (was this empty
state truly first-run vs. filtered?). Those are MAY rules or
design-review rules, not lint rules. The skill should distinguish the
two tiers.

**Recommendation**: define a rule-format YAML / frontmatter that
captures *selector, scope, severity, lintable-yes/no, citation*,
separate from the human-readable prose body. The skill renders the
prose version into the doc and the YAML version into a lint config.

---

## 7. The doc must be a governed artefact

Like recipes and skills, `docs/UX_DESIGN.md` should only be edited by
**humans in interactive sessions**. Automated agents may suggest
changes via issue/PR comments, but never edit the doc directly.
Reasons:

- A drift-prone doc is worse than no doc — agents start ignoring it.
- The doc encodes decisions; decisions are human.
- The exception list especially must not grow without human consent;
  every exception is a small concession to convention drift.

This mirrors the recipe and skill editing rules in the existing
RULEBOOK. The framework already has the pattern — extend it to
`docs/UX_DESIGN.md`.

**Recommendation**: add `docs/UX_DESIGN.md` to the editability table
in RULEBOOK.md alongside skills and recipes, with the same
human-interactive-only rule.

---

## 8. Companion artefacts and what each contributes

The skill should know what *not* to put in UX_DESIGN.md by being clear
about what other artefacts contribute:

| Artefact | Owns |
|---|---|
| `theme.ts` (or equivalent) | Hex values, spacing scale, typography scale — the *what* of design tokens |
| `docs/UX_DESIGN.md` | The *rules* for using tokens — when, where, with what intent |
| Figma / per-feature `UX_DESIGN.md` | Pixel-perfect screen layouts — the *visual specs* |
| Component inventory (Storybook) | The *available primitives* — what's been built and is reusable |
| `docs/ARCHITECTURE.md` | Routing, data flow, system shape — *non-visual* |

A `docs/UX_DESIGN.md` that re-asserts hex values is wrong; that draws
screen mocks is wrong; that describes routes is wrong. Each artefact's
boundary should be stated in the file's preamble.

The existing per-feature `ux-design` skill should be updated so its
output explicitly defers visual / interaction *rules* to the
project-wide `docs/UX_DESIGN.md`, captured by reference rather than
re-derivation. The per-feature artefact then focuses on *what's unique
to this feature* — screens, states, copy, validation specifics — and
inherits everything else.

---

## 9. Suggested structure for the project-wide skill

A skeleton, mirroring the section-gate pattern of the existing skill:

### Trigger

`triggers: human-interactive`. Like `solution-architecture`, this skill
runs at project birth (and on demand to extend) and is not part of the
automated pipeline.

### Preconditions

- Repo has `docs/ARCHITECTURE.md` (the project-wide UX spec sits
  alongside it).
- Repo has at least minimal frontend scaffolding (theme file,
  component library chosen, one or two screens). A greenfield repo
  with no UI can defer this skill.

### Mode select

- **Greenfield**: project has no UI yet. Walk through decision-axes
  and best-practice baseline; produce a forward-looking spec.
- **Brownfield**: project has existing UI. Run the six-step bootstrap
  procedure (§4 above): audit → identify conflicts → score against
  best practice → extract rules → capture exceptions → file
  Requirement issue.

### Section walk (the artefact body)

For each section, the skill prompts the human for:
- The project's **position on the decision axis** (with the
  recommended default offered).
- Project-specific **token names** (palette keys, design-token
  identifiers).
- Any **intentional deviations** from the baseline that should be
  captured as exceptions.

Suggested section order (matches what worked in ocs-testbench):

1. Foundations (color, typography, spacing, radius, iconography)
2. Layout & navigation
3. Forms
4. Buttons (with destructive-action sub-section as a focus area —
   it's the most error-prone)
5. Tables and lists
6. Modals
7. Cards and panels
8. Feedback (toasts, banners, error screens, status indicators)
9. Data display
10. Dark mode (if applicable)
11. Accessibility (a11y) — default WCAG AA
12. Exceptions
13. Appendices (helpers, references)

### Persistence

- Write `docs/UX_DESIGN.md` on the current branch.
- Optionally update `docs/ARCHITECTURE.md` to link the new doc.
- In brownfield mode: open a GitHub Requirement issue listing the
  inconsistencies to fix, each citing a rule ID.
- Commit but do not push — human controls push and PR.

### Exit hands-to

`human: review the doc, push the branch, open a PR; the
brownfield-Requirement issue will flow through the normal pipeline`.

---

## 10. Things we didn't tackle but the framework should consider

- **Internationalisation (i18n)**. Strings, RTL layout, locale-sensitive
  number/date formatting. The skill should at minimum require the
  project to declare *whether i18n is in scope* and provide a baseline
  ruleset if yes.
- **Print styles / export views**. Many ops tools eventually need
  printable reports.
- **Performance budgets**. *"Initial load < 200kB"* is a UX concern that
  rarely makes it into design docs but should.
- **Telemetry / analytics conventions**. What events fire, what's
  named what — drift-prone, hard to reverse later.
- **Versioning the doc itself**. `docs/UX_DESIGN.md` will change; a
  migration log helps when an older PR is reviewed against the rules
  of its time.
- **Light / dark / high-contrast modes** as a first-class section, not
  buried in foundations.

---

## 11. What worked in the writing process

For posterity / process documentation:

1. **Doing the audit first surfaced the right questions.** Going in
   with a structure and trying to fill it produces generic prose;
   going in with the audit produces concrete, defensible rules.
2. **The "must-see vs. safe-to-miss" framing for toasts** emerged from
   user dialogue, not from any best-practice canon. The doc is better
   for capturing the reasoning in the user's voice. The skill should
   actively prompt for these axis-defining decisions during bootstrap
   rather than offering a default and moving on.
3. **The exception case (peer-status auto-close) tested the framing.**
   Walking through whether it was a bug or a sanctioned exception was
   the most valuable conversation in the whole process — it forced
   the underlying rule to be re-stated more precisely. The skill
   should *seek out* candidate exceptions during bootstrap, not wait
   for them to surface.
4. **Splitting the rules from the bug list into two artefacts.**
   Mixing them in the doc would have created a moving target ("which
   deviations have been fixed?") and weakened the doc's authority.
5. **Best-practice-first, code-reality-second.** The temptation to
   enshrine current code as canonical is real; resisting it produced
   a Requirement issue with 26 fix items but a doc that's actually
   worth following.

---

## 12. Proposed framework changes (concrete)

If these notes are adopted, the framework changes would be:

1. **Split or extend `skills/ux-design`** to handle the project-wide
   artefact in addition to the per-feature one. Recommend two
   separate skills: `ux-spec` (project-wide) and `ux-feature-design`
   (per-feature, what the existing skill does).
2. **Add `concepts/ux-design-philosophy.md`** explaining the three-
   artefact model (baseline / project-wide / per-feature) and the
   decision-axis approach.
3. **Add `skills/ux-spec/baseline.md`** containing the universal
   best-practice rules every project inherits.
4. **Add `docs/UX_DESIGN.md` to the RULEBOOK editability table** with
   `human-interactive-only` semantics.
5. **Add `docs/UX_DESIGN.md` as a precondition** for the per-feature
   `ux-design` skill — so the per-feature handoff inherits the
   project-wide rules instead of re-deriving them.
6. **Consider an `ux-lint` recipe** that scans the codebase against
   the lintable subset of UX_DESIGN.md rules. Long-term value.

---

## Reference: the file we produced

`docs/UX_DESIGN.md` in `eddiecarpenter/ocs-testbench` (branch
`docs/ux-design`) is a worked example of the project-wide artefact
this skill should produce. It is the result of running the procedure
in §4 manually. Use it as a shape reference; the rule content is
project-specific.
