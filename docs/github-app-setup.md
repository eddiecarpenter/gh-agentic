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
| `projects` | write | Move cards on the GitHub Project |
| `metadata` | read | Always required by GitHub |
| `secrets` | **NOT INCLUDED** | See rationale below |

### `secrets: write` exclusion — rationale

The legacy `setup-claude-auth` composite action used `goose-agent`'s PAT to call `gh secret set CLAUDE_CREDENTIALS_JSON`. That required `secrets: write`.

The App **does not** carry `secrets: write`, deliberately. The App already holds `contents: write` (i.e. can rewrite anything in the repo). Adding `secrets: write` on top would make a leaked App key a clean privilege-escalation primitive — an attacker could swap the Claude credentials for their own and harvest every subsequent pipeline run.

The trade: `CLAUDE_CREDENTIALS_JSON` refresh becomes a manual operation. A human with appropriate access runs `gh secret set CLAUDE_CREDENTIALS_JSON --repo eddiecarpenter/gh-agentic < credentials.json` when the credentials need updating. This is rare, and the security posture is worth it.

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

The App's identity splits into one **variable** (not sensitive) and one **secret** (sensitive). Treating them according to their actual sensitivity is intentional — see "App ID is a variable, not a secret" in the Rationale section below.

### Repo variable

| Variable | Value |
|---|---|
| `AGENTIC_APP_ID` | The App ID integer (e.g. `1234567`). Public per GitHub's API; not sensitive. |

```bash
gh variable set AGENTIC_APP_ID --repo eddiecarpenter/gh-agentic --body "<app-id-integer>"
```

### Repo secret

| Secret | Value |
|---|---|
| `AGENTIC_APP_PRIVATE_KEY` | The full PEM contents — `-----BEGIN RSA PRIVATE KEY-----` through `-----END RSA PRIVATE KEY-----` inclusive, including newlines |

```bash
gh secret set AGENTIC_APP_PRIVATE_KEY --repo eddiecarpenter/gh-agentic < /path/to/downloaded-key.pem
```

The private key is stored as **raw PEM** — no base64 wrapping. `actions/create-github-app-token` accepts this format natively. After the secret is set, securely delete the local `.pem` file (`shred -u` or equivalent).

### Existing Claude credential secret

`CLAUDE_CREDENTIALS_JSON` remains as before. The App holds `secrets: write` (see Permissions section), which preserves the in-band credential refresh flow performed by `setup-claude-auth/action.yml`.

### Webhook fields

The App has no backend service. When creating the App, leave both **Webhook URL** and **Webhook secret** blank (or untick "Active"). Nothing on GitHub will deliver a webhook anywhere.

### Why App ID is a variable, not a secret

The App ID is a small public integer that GitHub displays on the App's settings page; anyone who knows the App's slug can resolve it via the public REST API. Marking it as a secret would dilute the meaning of "secret" (which should mean *"leaking this is harmful"*) and add audit overhead. The private key alone is what authorizes API calls — the App ID without the private key is useless.

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
