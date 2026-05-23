# Migration — `goose-agent` PAT identity replaced by the agentic GitHub App

**Status:** Breaking change. Required action for every domain repo before
upgrading to the framework version that ships feature [#622].

## What changed

The pipeline's service identity has been swapped out. The legacy `goose-agent`
secondary user account — authenticated in workflows via
`secrets.GOOSE_AGENT_PAT` and surfaced in the repo variable `AGENT_USER` —
has been replaced by a first-class **GitHub App** (`gh-agentic-pipeline`).
Workflows now mint short-lived installation tokens at runtime via
`actions/create-github-app-token@v1` using the App ID and private key, and
commit authorship changes from the PAT user to `<app-name>[bot]`.

This is a **one-way migration by design**. The `goose-agent` account is kept
dormant for one release cycle after the App ships (per Feature [#622]) so a
human can manually reinstate the PAT model if absolutely required, but there
is no `gh agentic migrate rollback` command and the framework does not
support automated rollback.

Context for the App itself — its manifest, permissions, installation
procedure, secret storage, and key rotation — lives in
[`docs/github-app-setup.md`](../docs/github-app-setup.md). This guide does
not duplicate that material; it references it.

Context for the sibling `GOOSE_PROVIDER` / `GOOSE_MODEL` →
`AGENT_PROVIDER` / `AGENT_MODEL` variable rename lives in
[`docs/migration-agent-vars-rename.md`](../docs/migration-agent-vars-rename.md).
The two migrations are siblings under parent #621 and are typically done
together.

## What did **not** change

- `CLAUDE_CREDENTIALS_JSON` remains a repo secret, now consumed by the
  `setup-agent-auth` action (which absorbed the Claude decode + validate
  + rotate flow that used to live in the retired `setup-claude-auth`
  action). The App is granted `secrets: write` precisely so the existing
  in-band credential refresh continues to work — see
  [`docs/github-app-setup.md`](../docs/github-app-setup.md) §"`secrets: write`
  rationale" for the trade-off.
- The pipeline workflows' overall shape, recipes, skills, and labels are
  unchanged. Only the identity minting the tokens has moved.
- `PROJECT_PAT` is a separate credential for Projects v2 mutations when the
  App is installed on a personal account. It is not part of this migration
  — see [`docs/github-app-setup.md`](../docs/github-app-setup.md)
  §"`PROJECT_PAT`".

## Prerequisites

Before starting the migration, confirm:

- The agentic pipeline GitHub App exists and is owned by the framework
  maintainer. Domain repos do **not** create their own App — they install
  the existing one.
- The `gh` CLI (v2.40+) is installed and authenticated against an account
  with **admin** rights on the domain repo (collaborator management, secret
  management, and variable management all require admin).
- You can reach the App's installation URL in a browser (required for the
  one-click install flow).
- You have write access to the GitHub Project that tracks the domain repo's
  pipeline cards, if project sync is in use.

## Required action — domain repos

The 8 steps below are the cutover. Run them in order. Each step has an
exact command (where applicable) and a success criterion to tick off before
moving on. **Do not** run `gh agentic upgrade <new-version>` until steps 1–3
are complete — doing so will break your pipeline on the next workflow run.

### 1. Install the agentic GitHub App on the target repo

The recommended path is `gh agentic init` (for first-time setup) or
`gh agentic project join` (for a repo that is already part of an agentic
project). Both commands detect whether the App is installed on the target
repo and walk you through installation when it is not, per Feature #625.

```bash
# First-time setup:
gh agentic init

# Existing agentic repo joining an existing project:
gh agentic project join
```

If you prefer to install manually — or the CLI helper is unavailable —
follow the manual steps in
[`docs/github-app-setup.md`](../docs/github-app-setup.md) §"Install the App
on `<owner>/<repo>`".

**Success criterion:** the App appears under
`https://github.com/<owner>/<repo>/settings/installations` with the
agentic pipeline App listed and access granted to this repo.

### 2. Set credentials (`AGENTIC_APP_ID` and `AGENTIC_APP_PRIVATE_KEY`)

If your repo inherits these from its organisation (organisation-scoped
variable and secret), skip this step. Verify inheritance by running the
`gh variable list` and `gh secret list` commands below and confirming
`AGENTIC_APP_ID` and `AGENTIC_APP_PRIVATE_KEY` resolve at the org level —
if so, you are done with this step.

Otherwise, set them at repo scope. The App ID is a non-sensitive integer;
the private key is the full PEM file contents.

```bash
# App ID — a small integer visible on the App's settings page.
gh variable set AGENTIC_APP_ID \
  --repo <owner>/<repo> \
  --body "<app-id-integer>"

# Private key — raw PEM, not base64-wrapped. Pipe the .pem file contents in.
gh secret set AGENTIC_APP_PRIVATE_KEY \
  --repo <owner>/<repo> \
  < /path/to/downloaded-key.pem
```

For organisation scope, substitute `--org <org>` for `--repo <owner>/<repo>`.

See [`docs/github-app-setup.md`](../docs/github-app-setup.md) §"Configuration
storage" for the rationale on why App ID is a variable and the key is a
secret.

**Success criterion:**

```bash
gh variable list --repo <owner>/<repo> | grep -q AGENTIC_APP_ID
gh secret   list --repo <owner>/<repo> | grep -q AGENTIC_APP_PRIVATE_KEY
```

Both greps return zero exit status, or the values resolve at the org scope.

### 3. Mount the framework version that ships the App swap

With the App installed and credentials configured, mount the new framework
version that contains the workflow changes:

```bash
gh agentic upgrade <new-version>
```

This overwrites `.agents/` in the domain repo with the new framework, including
the workflow files that mint App installation tokens instead of reading
`GOOSE_AGENT_PAT`. Commit the refreshed `.agents/` per the domain repo's normal
sync procedure.

**Success criterion:** `git log -1 .agents/` shows the mount commit and
`grep -r 'AGENTIC_APP_ID' .agents/` returns at least one match (typically in
the pipeline workflow template).

### 4. Verify the pipeline runs end-to-end

Run a no-op pipeline cycle against the domain repo to confirm the new
identity works. A throwaway Requirement is the simplest path: open one,
take it through scoping → design → dev → review, then close. Alternatively,
manually re-trigger an existing pipeline workflow via `workflow_dispatch`.

While the workflow runs, confirm the following from the run log and the
resulting commit:

```bash
# a) Installation token minting succeeded — look for this step in the log:
#       "Mint GitHub App token"
#    It must exit 0 and print a masked token.

# b) Commit authorship shows the App's bot identity.
git log -1 --format='%an <%ae>' origin/feature/<latest-feature>
# Expected: <app-name>[bot] <NNNNNN+<app-name>[bot]@users.noreply.github.com>

# c) No legacy PAT references remain in the workflow files.
! grep -rn 'GOOSE_AGENT_PAT' .github/

# d) No legacy AGENT_USER references remain in the workflow files.
! grep -rn 'AGENT_USER' .github/
```

**Success criterion:** checks (a)–(d) all pass. A green workflow run alone
is **not** sufficient — verify the App attribution on (b) explicitly,
because a run can go green using stale tokens if step 3 was incomplete.

### 5. Remove `goose-agent` as a collaborator

With the App fully driving the pipeline, the legacy user account is no
longer needed. Remove it from the repo's collaborator list:

```bash
gh api -X DELETE repos/<owner>/<repo>/collaborators/goose-agent
```

**Success criterion:**

```bash
gh api repos/<owner>/<repo>/collaborators/goose-agent 2>&1 | grep -q 'Not Found'
```

### 6. Remove `goose-agent` from the GitHub Project

If `goose-agent` was added as a member of the GitHub Project tracking this
repo's pipeline, remove it. This must be done in the Projects v2 UI (or
via the GraphQL API) — there is no one-liner `gh` command for project
membership.

1. Open the Project in the GitHub UI.
2. *Settings → Manage access*.
3. Locate `goose-agent` in the member list and click **Remove**.

**Success criterion:** `goose-agent` no longer appears under Project *Settings → Manage access*.

Skip this step cleanly if `goose-agent` was never a project member.

### 7. Delete the `GOOSE_AGENT_PAT` secret

With the App minting tokens and the legacy account removed, the old PAT
secret is dead weight. Delete it:

```bash
gh secret delete GOOSE_AGENT_PAT --repo <owner>/<repo>
```

**Success criterion:**

```bash
! gh secret list --repo <owner>/<repo> | grep -q GOOSE_AGENT_PAT
```

### 8. Remove the `AGENT_USER` repo variable

`AGENT_USER` was the repo variable that told workflows which GitHub user
the `goose-agent` PAT belonged to. With the App identity there is no such
user — commit authorship comes from the App's bot identity, which the
workflow resolves from the installation token, not from a variable. Remove
it:

```bash
gh variable delete AGENT_USER --repo <owner>/<repo>
```

**Success criterion:**

```bash
! gh variable list --repo <owner>/<repo> | grep -q AGENT_USER
```

## Post-migration verification

Run the full verification checklist below after completing all 8 steps.
This is the consumer-repo adaptation of the end-to-end checklist in
[`docs/github-app-setup.md`](../docs/github-app-setup.md) §"End-to-end
verification checklist".

```bash
# 1. Confirm zero PAT references remain in workflows on this repo.
! grep -rn 'GOOSE_AGENT_PAT' .github/

# 2. Confirm zero AGENT_USER references remain in workflows on this repo.
! grep -rn 'AGENT_USER' .github/

# 3. Confirm the latest pipeline-produced commit is App-attributed.
git log -1 --format='%an <%ae>' origin/feature/<latest-feature>
# Expected: <app-name>[bot] <NNNNNN+<app-name>[bot]@users.noreply.github.com>

# 4. Confirm installation token minting works in the most recent workflow run.
#    In the GitHub Actions UI, open the latest pipeline run and verify
#    the "Mint GitHub App token" step exited 0.

# 5. Confirm the legacy secret and variable are gone.
! gh secret   list --repo <owner>/<repo> | grep -q GOOSE_AGENT_PAT
! gh variable list --repo <owner>/<repo> | grep -q AGENT_USER

# 6. Confirm goose-agent is no longer a collaborator.
gh api repos/<owner>/<repo>/collaborators/goose-agent 2>&1 | grep -q 'Not Found'
```

If all six checks pass, the domain repo's migration is complete.

## Rollback

There is no automated rollback. **This migration is one-way by design.**
There is no `gh agentic migrate rollback` command, and the framework will
not provide one.

Per Feature [#622], the `goose-agent` account is kept dormant for one
release cycle after the App ships. During that window, a human operator
can manually reinstate the PAT model on a domain repo by reversing steps
5, 7, and 8 (re-adding the collaborator, re-creating the `GOOSE_AGENT_PAT`
secret from a freshly generated PAT, and re-creating the `AGENT_USER`
variable) and installing the prior framework version with `gh agentic upgrade
<prior-version>`. This is a manual operation with no framework-side
automation and is supported only for genuine emergencies.

After that one-cycle grace window, the `goose-agent` account is retired
permanently and rollback is no longer possible without restoring a
secondary GitHub user account from scratch — which violates GitHub's Terms
of Service and is explicitly not supported.

## Failure mode if skipped

If a consumer upgrades to the framework version that ships the App swap
without performing steps 1–3 first, the pipeline will fail to mint
installation tokens and every workflow run will error on the first
`actions/create-github-app-token` step — typically with
`Error: Input required and not supplied: app-id` or
`Error: Input required and not supplied: private-key` if the secrets are
missing, or `Error: HttpError: Not Found` if the App is not installed on
the repo.

Subsequent steps in the workflow that depend on `steps.app-token.outputs.token`
will all fail-fast. The workflow run turns red immediately and no commits,
issues, or PRs are produced. The failure is loud, not silent — which is
the intended behaviour.

Recovery: complete steps 1–3 (install the App, set the credentials, re-run
`gh agentic upgrade` if needed to realign `.agents/` with the installed App) and
re-trigger the failed workflow. No data is lost; the pipeline resumes
from the failed run.

[#621]: https://github.com/eddiecarpenter/gh-agentic/issues/621
[#622]: https://github.com/eddiecarpenter/gh-agentic/issues/622
[#625]: https://github.com/eddiecarpenter/gh-agentic/issues/625
