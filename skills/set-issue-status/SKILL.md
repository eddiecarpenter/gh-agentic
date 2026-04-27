---
name: set-issue-status
description: Sets the Status field on a GitHub ProjectV2 item for a given issue, finding or creating the project item if needed and resolving the target status name to its option ID at runtime so the caller does not have to deal with GraphQL plumbing. Use when a calling skill needs to transition an issue's project status as part of a phase change — e.g., a Requirement moving to "Scoping", a Feature moving to "In Design", a Feature moving to "Done". Use even when the calling skill doesn't say "set status" — phrases like "transition the requirement to scoping", "move the feature to in-design", "mark the issue scheduled" should trigger this primitive.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
emits-exit-block: false
---

# Set Issue Status

## Goal

Set the Status field on a GitHub ProjectV2 item for a given issue,
finding or creating the project item as needed, and resolving the
target status name to its option ID at runtime. The caller passes
the issue number and a human-readable status name (e.g., `"Scoping"`,
`"In Design"`); this primitive handles every GraphQL ID resolution
internally.

## Output Artefacts

- Updated Status field value on the named issue's project item —
  observable on the GitHub Project board after the call.
- A return value to the caller of shape:
  ```
  { repo: <string>, issue: <int>, project_id: <string>,
    item_id: <string>, status: <string>, set: <bool> }
  ```
  `set: true` on success; on failure this primitive raises rather
  than returning `set: false`. `item_id` is the project item's ID
  (newly created if the issue wasn't already on the board, or the
  existing one). `status` is echoed back verbatim from the caller's
  request so the caller can verify match-case if they need to.

No file artefacts. No mutation outside the named project item.

## Definitions

- `skills/definitions/error-handling.md` — the severity taxonomy
  used to classify the failure modes detected in steps 1–5
  (`PROJECT_ID_MISSING` is `ERROR`; `ISSUE_NOT_FOUND` is `ERROR`;
  `STATUS_FIELD_NOT_FOUND` is `ERROR`; `STATUS_OPTION_NOT_FOUND`
  is `ERROR`; `GH_API_FAILED` is `ERROR`).

## Dependencies

None. This primitive shells out to `gh api graphql` and `gh variable
get`; it does not invoke any other skill.

## Steps

1. **Receive the inputs from the caller.** Required fields:
   - `repo` (string, `owner/name` format) — e.g.,
     `eddiecarpenter/gh-agentic`.
   - `issue` (int) — the issue number whose project status is being
     set.
   - `status` (string) — the target status name, matched
     case-insensitively against the project's Status field options
     (e.g., `"Scoping"`, `"In Design"`, `"Done"`).

2. **Resolve `AGENTIC_PROJECT_ID`.** Read the repo variable that
   names the GitHub ProjectV2 this repo is bound to:

   ```bash
   AGENTIC_PROJECT_ID=$(gh variable get AGENTIC_PROJECT_ID --repo "$repo")
   ```

   **Detect:**
   - Exit code non-zero, or output empty/whitespace → raise
     `PROJECT_ID_MISSING` with severity `ERROR`. The repo is not
     configured for the agentic pipeline; the caller should
     surface a recommendation to run `gh agentic init` or
     `gh agentic project join`.

3. **Resolve the issue's node ID.** GraphQL operates on node IDs,
   not issue numbers. Convert:

   ```bash
   gh api graphql -f query='
     query($owner:String!, $name:String!, $num:Int!) {
       repository(owner:$owner, name:$name) {
         issue(number:$num) { id }
       }
     }' \
     -F owner="${repo%/*}" \
     -F name="${repo#*/}" \
     -F num="$issue" \
     --jq '.data.repository.issue.id'
   ```

   **Detect:**
   - Output empty (the issue number doesn't exist in the repo) →
     raise `ISSUE_NOT_FOUND` with severity `ERROR`.
   - Exit code non-zero → raise `GH_API_FAILED` with severity
     `ERROR`; include the stderr.

   Hold the result as `<issue-node-id>`.

4. **Find or create the project item for this issue.** First, check
   if the issue is already on the project board:

   ```bash
   gh api graphql -f query='
     query($id:ID!) {
       node(id:$id) {
         ... on Issue {
           projectItems(first:20) {
             nodes { id project { id } }
           }
         }
       }
     }' \
     -F id="<issue-node-id>" \
     --jq '.data.node.projectItems.nodes[] | select(.project.id == "'"$AGENTIC_PROJECT_ID"'") | .id'
   ```

   - If the query returns an item ID → reuse it as `<item-id>`.
   - If it returns nothing → the issue isn't on the board yet.
     Add it:

     ```bash
     gh api graphql -f query='
       mutation($projectId:ID!, $contentId:ID!) {
         addProjectV2ItemById(input:{projectId:$projectId, contentId:$contentId}) {
           item { id }
         }
       }' \
       -F projectId="$AGENTIC_PROJECT_ID" \
       -F contentId="<issue-node-id>" \
       --jq '.data.addProjectV2ItemById.item.id'
     ```

     Hold the returned ID as `<item-id>`.

   **Detect:**
   - Either query failing with non-zero exit code → raise
     `GH_API_FAILED` with severity `ERROR`.
   - The mutation succeeding but returning no ID → raise
     `GH_API_FAILED` (the project may have been deleted since
     `AGENTIC_PROJECT_ID` was set; recommend `gh agentic check`).

5. **Resolve the Status field ID and the target option ID.** The
   project's Status field is a `ProjectV2SingleSelectField` with a
   set of named options. Query both at once:

   ```bash
   gh api graphql -f query='
     query($projectId:ID!) {
       node(id:$projectId) {
         ... on ProjectV2 {
           fields(first:50) {
             nodes {
               ... on ProjectV2SingleSelectField {
                 id name
                 options { id name }
               }
             }
           }
         }
       }
     }' \
     -F projectId="$AGENTIC_PROJECT_ID"
   ```

   From the response, locate the field whose `name` is `"Status"`
   (case-insensitive match). Within that field's `options`, locate
   the option whose `name` matches the caller's `status` value
   (case-insensitive).

   **Detect:**
   - No field named `Status` → raise `STATUS_FIELD_NOT_FOUND` with
     severity `ERROR`. The project is not configured with a Status
     field; recommend `gh agentic repair` or manual project setup.
   - No option matching the caller's `status` value → raise
     `STATUS_OPTION_NOT_FOUND` with severity `ERROR`. List the
     available option names in the error detail so the caller
     (or human) can correct the status string. Note: this primitive
     does NOT auto-create option values — adding a status option is
     a project-configuration concern, not a runtime concern.

   Hold the resolved values as `<status-field-id>` and
   `<status-option-id>`.

6. **Set the Status field value.** Single mutation, atomic:

   ```bash
   gh api graphql -f query='
     mutation($projectId:ID!, $itemId:ID!, $fieldId:ID!, $optionId:String!) {
       updateProjectV2ItemFieldValue(input:{
         projectId:$projectId,
         itemId:$itemId,
         fieldId:$fieldId,
         value:{singleSelectOptionId:$optionId}
       }) {
         projectV2Item { id }
       }
     }' \
     -F projectId="$AGENTIC_PROJECT_ID" \
     -F itemId="<item-id>" \
     -F fieldId="<status-field-id>" \
     -F optionId="<status-option-id>"
   ```

   **Detect:**
   - Exit code non-zero → raise `GH_API_FAILED` with severity
     `ERROR`; include the stderr. The mutation is idempotent — the
     same call repeated with the same option leaves the field
     unchanged — so the caller may retry blindly on transient
     network failures.

7. **Return the result to the caller.**

   ```
   { repo: "<repo>", issue: <issue>, project_id: "<AGENTIC_PROJECT_ID>",
     item_id: "<item-id>", status: "<caller-supplied status>",
     set: true }
   ```

## Verification

Per `skills/definitions/verification-procedure.md` "Section format".
Skill-specific commands:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/set-issue-status/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/set-issue-status/SKILL.md
```

Pass criteria: both commands exit 0.
## Error Handling

- `PROJECT_ID_MISSING` from step 2 → severity `ERROR`; propagate.
  Repo is not configured for the agentic pipeline; recommend
  `gh agentic init` or `gh agentic project join`.
- `ISSUE_NOT_FOUND` from step 3 → severity `ERROR`; propagate.
  Caller passed a bad issue number, or the issue exists in a
  different repo than `repo`.
- `STATUS_FIELD_NOT_FOUND` from step 5 → severity `ERROR`;
  propagate. The project is missing the Status field; recommend
  `gh agentic repair`.
- `STATUS_OPTION_NOT_FOUND` from step 5 → severity `ERROR`;
  propagate. List the available option names in the error so the
  caller (or human) can correct the status string. Do not
  auto-create options — that's a project configuration concern.
- `GH_API_FAILED` from steps 3–6 → severity `ERROR`; propagate.
  The caller decides whether to retry — this primitive does not
  implement retry because the right policy depends on the caller's
  context (rate limit vs auth vs network).
- All other errors: propagate (default).
