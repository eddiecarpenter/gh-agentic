# Migration — `GOOSE_*` configuration variables renamed to `AGENT_*`

**Status:** Breaking change. Required action for every domain repo before
upgrading to the framework version that ships feature [#624].

## What changed

Two repo/org-scoped GitHub Actions configuration variables have been renamed
to drop the legacy `GOOSE_*` prefix and reflect the framework's current
agent-agnostic identity:

| Old name          | New name           | Default value (used when unset) |
| ----------------- | ------------------ | ------------------------------- |
| `GOOSE_PROVIDER`  | `AGENT_PROVIDER`   | `claude-code`                   |
| `GOOSE_MODEL`     | `AGENT_MODEL`      | `default`                       |

The variables select the agent provider and model the framework workflows
hand to Goose at runtime. They are **GitHub Actions configuration
variables** (set under *Settings → Secrets and variables → Actions →
Variables*), not environment variables on a developer machine.

After this version is mounted, the framework workflows look up
`vars.AGENT_PROVIDER` / `vars.AGENT_MODEL`. Any value still set under the
old names is silently ignored.

## What did **not** change

- `GOOSE_PROVIDER:` and `GOOSE_MODEL:` continue to appear as **environment
  variable names** on the left-hand side of the workflow `env:` blocks, and
  as **YAML keys** inside the `~/.config/goose/config.yaml` heredoc. These
  are the Goose CLI's own contract and must remain spelled exactly the way
  Goose expects. Only the right-hand `${{ vars.* }}` lookup changed.
- `GOOSE_AGENT_PAT` is **not** part of this rename — it is being removed
  separately by the GitHub App identity migration (feature #622).
- The default values (`claude-code`, `default`) are unchanged.

## Required action — domain repos

Run **before** running `gh agentic mount <new-version>`:

1. Read the current values you have set:

   ```bash
   gh variable get GOOSE_PROVIDER --repo <owner>/<repo>   # may print 'not found'
   gh variable get GOOSE_MODEL    --repo <owner>/<repo>   # may print 'not found'
   ```

   For organisation-scoped variables, use `gh api orgs/<org>/actions/variables/<NAME>`.

2. Set the new variables at **the same scope** (repo or org) as the old ones,
   using the same values. If you never set the old variables, the framework
   uses the defaults shown above and you may skip this step entirely.

   Repo scope:
   ```bash
   gh variable set AGENT_PROVIDER --repo <owner>/<repo> --body "<value-or-claude-code>"
   gh variable set AGENT_MODEL    --repo <owner>/<repo> --body "<value-or-default>"
   ```

   Organisation scope:
   ```bash
   gh variable set AGENT_PROVIDER --org <org> --body "<value>"
   gh variable set AGENT_MODEL    --org <org> --body "<value>"
   ```

3. Mount the new framework version:

   ```bash
   gh agentic mount <new-version>
   ```

4. Optionally remove the old variables once every consumer has been upgraded:

   ```bash
   gh variable delete GOOSE_PROVIDER --repo <owner>/<repo>
   gh variable delete GOOSE_MODEL    --repo <owner>/<repo>
   ```

## Failure mode if skipped

If the new variables are not set before the mount and you previously relied on
non-default values, the workflows fall back to the documented defaults
(`claude-code` / `default`). The pipeline will keep running but against the
default provider and model rather than the ones you intended — surprising,
not catastrophic. Set the new variables to recover.

[#624]: https://github.com/eddiecarpenter/gh-agentic/issues/624
