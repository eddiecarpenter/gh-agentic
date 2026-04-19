# Decisions

Architecture Decision Records (ADRs) for this repository. Additive log — new
entries at the top; existing entries immutable.

---

## ADR-001 — CATALOGUE.md does not emit `loads` (2026-04-19, #570)

**Context.** Task #577 of feature #570 ("Verify frontmatter, rebuild
CATALOGUE.md, confirm reference-skill classification") included the acceptance
criterion: *"CATALOGUE.md shows refactor-assessment in the loads of
feature-design, dev-session, and issue-session."* However,
`skills/build-catalogue.md` — the canonical procedure for generating the
catalogue — explicitly forbids emitting `loads` in the catalogue output:

> Do not emit body content, `loads`, `emits-exit-block`, or `exit-hands-to`.
> The catalogue is an index, not a frontmatter dump.

Task #577 also required *"Rebuild CATALOGUE.md by invoking
skills/build-catalogue.md"*. The two instructions are mutually exclusive as
written.

**Decision.** Follow `skills/build-catalogue.md` verbatim. The rebuild emits
`name`, `description`, and `triggers` only, grouped by `category` and sorted
alphabetically within each category — deterministic and byte-stable. The
`loads` relationship remains authoritative in each consumer skill's
frontmatter, which is what `session-init` actually reads at session start.

**Rationale.** `skills/build-catalogue.md` is the authoritative procedure;
deviating from it to satisfy a single task's AC would (a) introduce
non-determinism compared to prior rebuilds, (b) violate the
token-cost-first principle in `skills/skill-categories.md` (the catalogue
is an index, not a dump), and (c) silently diverge from the framework's
own rules. The spirit of the AC — that the `refactor-assessment → consumer`
relationship is declared and discoverable — is satisfied by the frontmatter
declarations in `skills/feature-design.md`, `skills/dev-session.md`, and
`skills/issue-session.md` (verified during task #577).

**Consequences.** If future auditing requires the catalogue to surface
`loads`, the right fix is to raise a follow-up scope change against
`skills/build-catalogue.md` (the authoritative procedure) rather than
patching the catalogue ad-hoc. Any such change must also update the
determinism rules and the "do not emit" rule in that file.
