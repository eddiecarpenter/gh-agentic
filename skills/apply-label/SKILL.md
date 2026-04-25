---
name: apply-label
description: Applies one or more labels to a GitHub issue or pull request and optionally removes conflicting labels in the same call so phase-state transitions are atomic from the caller's perspective. Returns the resulting label set so the caller can verify without a second round-trip. Use when a calling skill needs to add, remove, or swap labels on an issue/PR — phase transitions like `in-design`→`in-development`, marking PRs `ready-for-review` or `approved`, tagging during triage, or any label-driven workflow signal. Use even when the calling skill doesn't say "apply label" — phrases like "transition the issue to in-development", "mark the PR approved", "tag this triaged" should trigger this primitive.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
emits-exit-block: false
---

# Apply Label

## Goal

Apply one or more labels to a named GitHub issue or pull request,
optionally removing conflicting labels in the same operation, and
return the resulting label set so the caller can verify the
transition.

## Output Artefacts

- Updated label state on the named issue/PR — observable via
  `gh issue view <issue> --repo <repo> --json labels`. After the
  call, every label in the caller's `add` list is present; every
  label in the caller's `remove` list is absent.
- A return value to the caller of shape:
  ```
  { repo: <string>, issue: <int>, labels_after: [<string>, ...],
    added: [<string>, ...], removed: [<string>, ...], applied: <bool> }
  ```
  `applied: true` on success; on failure this primitive raises
  rather than returning `applied: false`. `added` and `removed`
  reflect the *effective* changes (labels that were not already in
  the desired state) — re-applying an already-present label leaves
  it out of `added`.

No file artefacts. No mutation outside the named issue/PR.

## Definitions

- `skills/definitions/error-handling.md` — the severity taxonomy
  used to classify the failure modes detected in steps 2–3
  (`LABEL_LIST_EMPTY` is `ERROR`; `ISSUE_NOT_FOUND` is `ERROR`;
  `LABEL_NOT_FOUND` is `ERROR`; `GH_API_FAILED` is `ERROR`).

## Dependencies

None. This primitive shells out to `gh`; it does not invoke any
other skill.

## Steps

1. **Receive the inputs from the caller.** Required fields:
   - `repo` (string, `owner/name` format) — e.g., `eddiecarpenter/gh-agentic`.
   - `issue` (int) — the issue or PR number. The `gh issue edit`
     command works uniformly for issues and PRs.
   - `add` (list of strings) — labels to add. May be empty only if
     `remove` is non-empty.
   - `remove` (list of strings, optional, default `[]`) — labels to
     remove in the same call.

   If both `add` and `remove` are empty (or absent), raise
   `LABEL_LIST_EMPTY` with severity `ERROR`. A no-op call is almost
   always a caller bug (forgot to populate the lists).

2. **Read the current label state.** This lets the skill compute
   the *effective* add/remove sets and return them to the caller —
   without it, the caller cannot tell which labels were already in
   the desired state.

   ```bash
   gh issue view "$issue" --repo "$repo" --json labels \
     --jq '[.labels[].name]' > /tmp/labels-before.json
   ```

   **Detect:**
   - Exit code non-zero with stderr containing "Could not resolve to
     an Issue" → raise `ISSUE_NOT_FOUND` with severity `ERROR`.
   - Exit code non-zero otherwise → raise `GH_API_FAILED` with
     severity `ERROR`; include the stderr in the error detail.

3. **Apply the label changes.** Use a single `gh issue edit` call so
   the add and remove happen in one API round-trip — separate calls
   would leave the issue momentarily in an intermediate state, which
   matters for label-driven workflow triggers.

   ```bash
   ADD_ARGS=$(printf -- '--add-label %s ' "${add[@]}")
   RM_ARGS=$(printf -- '--remove-label %s ' "${remove[@]}")
   gh issue edit "$issue" --repo "$repo" $ADD_ARGS $RM_ARGS
   ```

   **Detect:**
   - Exit code non-zero with stderr containing "not found" AND
     mentioning a label name → raise `LABEL_NOT_FOUND` with
     severity `ERROR`. The label does not exist in the repo. This
     primitive does not auto-create labels; the caller is expected
     to ensure repo label hygiene.
   - Exit code non-zero with stderr containing "Could not resolve to
     an Issue" → raise `ISSUE_NOT_FOUND` with severity `ERROR`
     (covers a TOCTOU race where the issue was deleted between
     step 2 and step 3).
   - Exit code non-zero otherwise → raise `GH_API_FAILED` with
     severity `ERROR`.

4. **Read the new label state.** Re-query rather than computing the
   expected set client-side, because GitHub's label-application
   semantics (e.g., interactions with required labels, repo
   automation rules) can produce a final state that differs from a
   naive `(before ∪ add) \ remove`.

   ```bash
   gh issue view "$issue" --repo "$repo" --json labels \
     --jq '[.labels[].name]' > /tmp/labels-after.json
   ```

5. **Compute the effective add/remove sets and return.** Subtract
   the before-set from the after-set for `added`; subtract the
   after-set from the before-set for `removed`. Build:

   ```
   { repo: "<repo>", issue: <issue>,
     labels_after: <after-set>,
     added: <after \ before>,
     removed: <before \ after>,
     applied: true }
   ```

## Verification

Run the framework checks against this skill:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/apply-label/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/apply-label/SKILL.md
```

Pass criteria: both commands exit 0.

### Mechanical checks

Run by `verify-skill-mechanical.py`:

- `all_sections_present` — every mandatory section heading exists.
- `frontmatter_required_fields(name, description, triggers, loads)`.
- `frontmatter_name_valid` — kebab-case, matches filename.
- `description_within_length_limit` — ≤ 1024 chars.
- `description_assertive` — contains "Use when" + assertive clause.
- `description_third_person`.
- `references_resolve` — every `loads:` path resolves to a file.

### Ground-truth checks

Run by `check-description-triggers.py`:

- `description_triggers_appropriately` — phrasings classified per
  the `GROUND_TRUTH` entry for `apply-label`.

## Error Handling

- `LABEL_LIST_EMPTY` from step 1 → severity `ERROR`; propagate.
  Caller bug — both `add` and `remove` were empty.
- `ISSUE_NOT_FOUND` from steps 2 or 3 → severity `ERROR`; propagate.
  Caller passed a bad issue number, wrong repo, or the issue was
  deleted between read and write.
- `LABEL_NOT_FOUND` from step 3 → severity `ERROR`; propagate. The
  caller is expected to ensure label hygiene at the repo level —
  this primitive does not auto-create missing labels because doing
  so would mask label-name typos and let workflow signals drift.
- `GH_API_FAILED` from steps 2–3 → severity `ERROR`; propagate.
  The caller decides whether to retry — this primitive does not
  implement retry because the right policy depends on the caller's
  context (rate limit vs auth vs network).
- All other errors: propagate (default).
