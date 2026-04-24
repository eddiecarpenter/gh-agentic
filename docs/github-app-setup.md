# GitHub App Setup

Reference for the **agentic pipeline GitHub App** — the service identity that replaces the legacy `goose-agent` user account and `GOOSE_AGENT_PAT` personal access token.

This document is the source of truth for the App's manifest, installation procedure, secret storage, key rotation, and post-install verification. Workflow files reference these decisions; do not change them in workflow PRs without amending this document first.

See Feature [#622](https://github.com/eddiecarpenter/gh-agentic/issues/622) for the migration's design context.

---

## Overview

The pipeline historically authenticated as `goose-agent`, a secondary GitHub user account whose personal access token was stored as `secrets.GOOSE_AGENT_PAT`. That model has three structural problems:

- Onboarding a new domain repo required adding `goose-agent` as a collaborator and project member — manual, easy to forget, silently broke pipelines.
- PATs require manual rotation. A secondary user account is also against GitHub's Terms of Service.
- The PAT had whatever permissions a write-collaborator has — broad, untyped, opaque.

The App is GitHub's first-class service-account primitive. Installing it on a repo grants exactly the permissions declared in its manifest, mints short-lived installation tokens automatically, and survives every individual user's account lifecycle. Commit authorship becomes `<app-name>[bot]`, distinct from any human author in the audit log.

The App is initially owned by `eddiecarpenter` (personal account). It can be transferred to an org later without disruption — App ID, private key, and existing installations all survive a transfer.

---

## Permissions and events

### Installation permissions

| Permission | Setting | Purpose |
|---|---|---|
| `contents` | write | Push commits, branches, files |
| `issues` | write | Manage Requirements, Features, Tasks |
| `pull_requests` | write | Open PRs, comment on reviews |
| `projects` (org) | write | Mutate Project v2 boards. **Ineffective for user-owned Projects v2** (platform limitation — see `PROJECT_PAT` below). |
| `repository_projects` | admin | Mutate legacy "Projects classic" boards attached to a repository. |
| `workflows` | write | Required for agent commits that touch `.github/workflows/*.yml`. Without it, dev-session pushes are rejected for any Task editing workflows. |
| `secrets` | write | In-band rotation of `CLAUDE_CREDENTIALS_JSON` (transitional — see rationale) |
| `metadata` | read | Always required by GitHub |

### `secrets: write` rationale (transitional)

`secrets: write` is included **as a pragmatic transitional measure**, not because it is the long-term right answer.

**Today.** Claude Code rotates its session credentials at runtime, and `setup-claude-auth/action.yml` writes the rotated credentials back to `CLAUDE_CREDENTIALS_JSON` via `gh secret set`. This requires `secrets: write` on the token. Removing the in-band rotation path would force a human to manually `gh secret set CLAUDE_CREDENTIALS_JSON ...` whenever Claude rotated — operationally painful for a workaround the framework was always going to outgrow.

**Production target.** Switch from Claude Code (with rotating session credentials) to a static **Anthropic API key**. API keys don't rotate in-band — they're set once and replaced only on intentional rotation. At that point, the `gh secret set` round-trip disappears entirely, `setup-claude-auth/action.yml` is replaced or trimmed, and **`secrets: write` should be removed from the App manifest** as a cleanup step.

**Security trade we are accepting transitionally.** A leaked App private key gains the ability to silently rewrite repo Actions secrets — no commit, no diff, no git-history audit trail. With `contents: write` only, credential exfiltration would require pushing a malicious workflow change (visible in `git log`), giving a chance for detection. With `secrets: write`, an attacker can swap `CLAUDE_CREDENTIALS_JSON` invisibly. This is acceptable while we are still in the Claude-Code-credentials phase because the credentials themselves are short-lived; it would not be acceptable indefinitely.

**Action when migrating to Anthropic API key:** open a Feature to remove `secrets: write` from the manifest, delete the in-band rotation logic from `setup-claude-auth`, and re-narrow the App's blast radius.

### Event subscriptions

| Event | Drives |
|---|---|
| `issues` | Issue Session, label-trigger workflows, project board sync |
| `pull_request_review` | PR Review Session |
| `label` | Pipeline phase transitions (`in-design`, `in-development`) |

---

## Installation

The App is created and installed once per environment. The procedure is a one-off human action.

### Create the App

1. Navigate to **Settings → Developer settings → GitHub Apps → New GitHub App** under the `eddiecarpenter` account.
2. Fill in:
   - **GitHub App name:** `gh-agentic-pipeline` (or chosen variant — must be globally unique on GitHub)
   - **Homepage URL:** `https://github.com/eddiecarpenter/gh-agentic`
   - **Webhook:** disabled (the App has no backend)
3. Set permissions per the table in **Permissions and events** above.
4. Set event subscriptions per the same section.
5. Set **Where can this GitHub App be installed?** to **Only on this account**.
6. Click **Create GitHub App**.
7. On the resulting App page, generate a private key (**Generate a private key** button). A `.pem` file downloads. **Store it securely** — it is never recoverable from GitHub if lost.
8. Note the **App ID** displayed on the App's settings page — a small integer.

### Install the App on `eddiecarpenter/gh-agentic`

1. From the App's settings page, click **Install App** in the left nav.
2. Choose `eddiecarpenter` as the install target.
3. Select **Only select repositories** → choose `gh-agentic`.
4. Click **Install**.

The App is now installed and can mint installation tokens for `gh-agentic` via `actions/create-github-app-token@v1`.

---

## Configuration storage

The App's identity splits into one **variable** (not sensitive) and one **secret** (sensitive). Treating them according to their actual sensitivity is intentional — see "Client ID is a variable, not a secret" below.

### Repo variable

| Variable | Value |
|---|---|
| `AGENTIC_APP_CLIENT_ID` | The App's **Client ID** (alphanumeric string like `Iv23li8K3O...`). Found on the App settings page under "About → Client ID". Not sensitive — the App's Client ID is public information surfaced wherever the App is installed. |

```bash
gh variable set AGENTIC_APP_CLIENT_ID --repo eddiecarpenter/gh-agentic --body "<client-id-string>"
```

**Why Client ID, not App ID.** GitHub's `actions/create-github-app-token` action deprecated `app-id` in favour of `client-id` (the OAuth-conventional identifier). Both still work for now, but the App ID input emits a deprecation warning on every run. Future versions of the action will remove `app-id` entirely; setting `AGENTIC_APP_CLIENT_ID` is forward-compatible.

### Repo secret

| Secret | Value |
|---|---|
| `AGENTIC_APP_PRIVATE_KEY` | The full PEM contents — `-----BEGIN RSA PRIVATE KEY-----` through `-----END RSA PRIVATE KEY-----` inclusive, including newlines |

```bash
gh secret set AGENTIC_APP_PRIVATE_KEY --repo eddiecarpenter/gh-agentic < /path/to/downloaded-key.pem
```

The private key is stored as **raw PEM** — no base64 wrapping. `actions/create-github-app-token` accepts this format natively. After the secret is set, securely delete the local `.pem` file (`shred -u` or equivalent).

### `PROJECT_PAT` (optional but strongly recommended)

GitHub Apps installed on a personal account **cannot mutate user-owned Projects v2** — confirmed limitation by GitHub Support ([community discussion #46681](https://github.com/orgs/community/discussions/46681)). The App can do everything else; project board sync alone needs a separate credential.

To keep the project board in lockstep with pipeline labels, set a **Personal Access Token** as the `PROJECT_PAT` secret:

```bash
gh secret set PROJECT_PAT --repo eddiecarpenter/gh-agentic --body "<your-pat>"
```

**Required scope:** the PAT needs read/write access to the user-owned Projects v2. For a fine-grained PAT: enable **Account permissions → Projects: Read and write**. For a classic PAT: the `project` scope.

**When unset:** every pipeline workflow that would update project status emits a `::warning::` and skips the project step cleanly (✓ outcome — no red noise). The pipeline itself works fully — labels carry pipeline state; only the visual project board stops auto-syncing. This is a documented, supported degradation mode.

**When set but invalid/expired:** the project step exits non-zero (visible red ✗ in the run summary), but `continue-on-error: true` keeps the overall workflow green. Refresh the PAT and project sync resumes on the next run.

**Rotation:** PATs do not auto-rotate. Plan for periodic refresh (annually if the PAT has an expiration, or on any suspected leak). When migrating to an organization topology (where the App's `Projects: Write` permission becomes effective), the `PROJECT_PAT` secret can be deleted entirely.

### Existing Claude credential secret

`CLAUDE_CREDENTIALS_JSON` remains as before. The App holds `secrets: write` (see Permissions section), which preserves the in-band credential refresh flow performed by `setup-claude-auth/action.yml`.

### Webhook fields

The App has no backend service. When creating the App, leave both **Webhook URL** and **Webhook secret** blank (or untick "Active"). Nothing on GitHub will deliver a webhook anywhere.

### Why Client ID is a variable, not a secret

The Client ID (and the legacy App ID) is a public identifier — GitHub displays it on the App's settings page, surfaces it wherever the App acts, and exposes it via the public REST API. Marking it as a secret would dilute the meaning of "secret" (which should mean *"leaking this is harmful"*) and add audit overhead with no security benefit. The private key alone is what authorizes API calls — the Client ID without the private key is useless.

---

## Private-key rotation

GitHub does not enforce expiration on App private keys. **Recommended cadence: annual.** Mandatory immediately on suspected compromise.

### Rotation procedure

1. **Generate a new key.** On the App's settings page, click **Generate a private key**. A new `.pem` file downloads. Both old and new keys are valid until step 4.

2. **Update the secret.**
   ```bash
   gh secret set AGENTIC_APP_PRIVATE_KEY --repo eddiecarpenter/gh-agentic < /path/to/new-key.pem
   ```

3. **Smoke-test the pipeline.** Trigger any workflow that mints an installation token (e.g. push a trivial commit, or manually run any `workflow_dispatch` workflow that uses the App). Confirm it succeeds with the new key.

4. **Revoke the old key.** Back on the App's settings page, click **Delete** next to the previous key entry. The old key now fails to mint tokens.

5. **Securely delete the local `.pem` file** (`shred -u` or equivalent).

### On compromise

If the private key is suspected leaked, perform steps 1–4 immediately, then audit recent App-attributed actions in `https://github.com/settings/installations` for unexpected activity.

---

## End-to-end verification checklist

After the App is installed, secrets are set, and workflow files are switched (Tasks #628, #629), confirm the swap end-to-end:

```bash
# 1. Confirm zero PAT references remain in workflows
! grep -rn 'GOOSE_AGENT_PAT' .github/

# 2. Confirm zero AGENT_USER references remain in workflows
! grep -rn 'AGENT_USER' .github/

# 3. Trigger a no-op pipeline cycle and confirm App attribution
#    (e.g. open a Requirement, take it through scoping → design → dev → review)
#    Then check the latest commit's author:
git log -1 --format='%an <%ae>' origin/feature/<latest-feature>
# Expected: gh-agentic-pipeline[bot] <NNNNNN+gh-agentic-pipeline[bot]@users.noreply.github.com>

# 4. Confirm installation token minting works (in any workflow run log)
#    Look for the "Mint GitHub App token" step succeeding.
```

If all four checks pass, the App identity swap is verified.
