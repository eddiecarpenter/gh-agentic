---
name: post-issue-comment
description: Posts a comment to a GitHub issue or pull request with the body the caller supplies, and returns the URL and ID of the created comment so the caller can verify or reference it. Use when a calling skill needs to publish text content tied to an issue or PR — Design Plans, iteration reports, PR review notes, status updates, or general issue-thread communication. Use even when the calling skill doesn't say "post comment" — phrases like "publish the plan", "report progress on the issue", "leave a note on the PR" should trigger this primitive.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
emits-exit-block: false
---

# Post Issue Comment

## Goal

Post a single comment to a named GitHub issue or pull request with
the caller's body verbatim, and return the comment's URL and ID.

## Output Artefacts

- A new comment on the named issue/PR — visible via
  `gh issue view <issue> --repo <repo> --comments`. The comment
  body matches the caller's `body` input character-for-character.
- A return value to the caller of shape:
  ```
  { url: <string>, id: <string>, posted: <bool>, repo: <string>, issue: <int> }
  ```
  `posted: true` on success; on failure this primitive raises
  rather than returning `posted: false`.

No file artefacts. No mutation outside the named issue/PR.

## Definitions

- `skills/definitions/error-handling.md` — the severity taxonomy
  used to classify the failure modes detected in step 4 below
  (`COMMENT_BODY_EMPTY` is `ERROR`; `GH_API_FAILED` is `ERROR`;
  `ISSUE_NOT_FOUND` is `ERROR`).

## Dependencies

None. This primitive is a single shell-out to `gh`; it does not
invoke any other skill.

## Steps

1. **Receive the inputs from the caller.** Required fields:
   - `repo` (string, `owner/name` format) — e.g., `eddiecarpenter/gh-agentic`.
   - `issue` (int) — the issue or PR number.
   - `body` (string, markdown allowed) — the comment text.

   If `body` is empty or whitespace-only, raise `COMMENT_BODY_EMPTY`
   with severity `ERROR`. Do not post empty comments — they are
   almost always a bug in the caller (forgot to populate `body`
   after constructing it).

2. **Write the body to a temp file.** Use a temp file rather than
   passing the body as a `--body` argument, because (a) shell
   quoting of multi-line markdown is fragile, and (b) GitHub API
   has length limits that the temp-file path handles cleanly.

   ```bash
   BODY_FILE=$(mktemp)
   printf '%s' "$body" > "$BODY_FILE"
   ```

3. **Post the comment.** Invoke `gh`:

   ```bash
   gh issue comment "$issue" --repo "$repo" --body-file "$BODY_FILE"
   ```

   The `gh` command works for both issues and pull requests — GitHub
   treats PR comments and issue comments uniformly via this command.
   Capture stdout (the comment URL) and the exit code.

   **Detect:**
   - Exit code non-zero with stderr containing "Could not resolve to
     an Issue" → raise `ISSUE_NOT_FOUND` with severity `ERROR`.
   - Exit code non-zero otherwise → raise `GH_API_FAILED` with
     severity `ERROR`; include the stderr in the error detail.
   - Exit code zero with empty stdout → raise `GH_API_FAILED`
     (anomaly: should always return the new comment URL).

4. **Extract the comment ID from the URL.** The URL has the form
   `https://github.com/<owner>/<name>/issues/<issue>#issuecomment-<id>`.
   Parse the trailing `<id>`:

   ```bash
   COMMENT_ID="${URL##*-}"
   ```

5. **Return the structured value to the caller.** Build:

   ```
   { url: "<URL>", id: "<COMMENT_ID>", posted: true, repo: "<repo>", issue: <issue> }
   ```

   Clean up the temp file: `rm -f "$BODY_FILE"`.

## Error Handling

- `COMMENT_BODY_EMPTY` from step 1 → severity `ERROR`; propagate.
  Caller bug.
- `ISSUE_NOT_FOUND` from step 3 → severity `ERROR`; propagate.
  Caller passed a bad issue number or wrong repo.
- `GH_API_FAILED` from step 3 → severity `ERROR`; propagate. The
  caller decides whether to retry — this primitive does not implement
  retry because the right retry policy depends on the caller's
  context (rate limit vs auth failure vs network blip).
- All other errors: propagate (default).
