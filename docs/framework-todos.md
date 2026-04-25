# Framework TODOs

Lightweight tracker for design decisions made during the skills-review
work that need to be implemented elsewhere in the framework. Each item
captures the decision, the affected components, and any context
needed to act on it later.

This file is for **framework-level** changes — changes to session-init,
recipes, CI workflows, the gh-agentic CLI, etc. — that flow from
decisions made while rebuilding the skills layer.

It is **not** a general issue tracker. For pipeline work that should
go through the normal Requirement → Feature → Design → Dev flow, open
a GitHub issue.

---

## Open

### Design CATALOGUE.md out of the framework

**Decision:** Drop the persisted `CATALOGUE.md` artefact. The agent
should build its skill metadata index in memory at session start by
walking `skills/` and `skills/definitions/` and reading frontmatter
from each file.

**Rationale:**

- Matches Anthropic's pattern (in-memory metadata loading at startup).
- Removes a maintenance burden — every skill creation, edit, or
  removal currently requires regenerating the catalogue.
- Eliminates staleness risk inherent in any persisted derived artefact.
- Simplifies skill-creator (no catalogue update step needed).
- Performance cost is negligible at our scale (~25–40 skills; sub-
  second to read all frontmatter at startup).

**Affected components:**

- `session-init` skill — currently reads `CATALOGUE.md` and auto-
  regenerates if stale. Needs to be rewritten to do in-memory
  loading instead.
- `gh agentic` CLI — `gh agentic check` includes catalogue rebuild
  logic. Either remove this command or repurpose it.
- `CATALOGUE.md` itself — the file at the repo root. Delete once
  nothing reads it.
- Any workflow or recipe that depends on `CATALOGUE.md` being
  current. Audit before deletion.

**Sequencing:** Do this when rewriting `session-init` under the new
skill spec. Removing `CATALOGUE.md` while session-init still depends
on it would break the framework.

**Source:** Discussion during skills-review (2026-04-25).

---

(Add new entries above this line as they emerge. Keep each entry
self-contained — anyone picking up the work later should be able to
act from the entry alone.)
