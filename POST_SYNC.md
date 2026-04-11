Adds a reusable build-and-test workflow for downstream repos, enforces HTTPS on tool installs, and improves release notes with a mandatory Downstream Actions section.

## Features
- Adds a `build-and-test.yml` reusable workflow with pinned action SHAs for Node.js 24 compatibility, distributed to downstream repos via `gh agentic sync`
- Adds a mandatory Downstream Actions section to release notes so consumers always know what post-sync steps are required

## Fixes
- Enforces HTTPS on all redirect-following installs with `--proto '=https' --tlsv1.2`
- Removes `build-and-test.yml` from `.github/workflows/` since it is a distribution-only template, not intended to run on this repo
- Skips the build-and-test job when `go.mod` is absent, preventing failures in non-Go repos
- Caches the `gh` CLI before checkout and skips redundant `apt-get` on cache hit, improving pipeline performance
- Enriches `recovery.md` with implementation context per completed task for better session continuity

## Downstream Actions
- New file `.ai/.github/workflows/build-and-test.yml` is available — downstream Go repos will receive it on next `gh agentic sync` and should verify their build passes with the new workflow
- Release notes now include a Downstream Actions section — no action required, but reviewers should expect it in future releases
