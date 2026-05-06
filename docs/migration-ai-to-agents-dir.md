# Migration: `.ai/` → `.agents/`

The framework directory has been renamed from `.ai/` to `.agents/` to align with
the emerging cross-harness standard for agent framework directories.

This is a one-time, mechanical change per repo. Follow the steps below.

---

## Why

`.agents/` is converging as the standard directory name used by multiple AI
harnesses (Claude Code, Goose, Copilot, and others) for their shared framework
files. Aligning now avoids a larger migration later and signals to contributors
which directory is agent-tooling rather than just "AI stuff".

---

## Steps per repo

Run these commands in each domain repo that mounts the framework:

```bash
# 1. Remove the old submodule cleanly
git submodule deinit .ai
git rm .ai
rm -rf .git/modules/.ai

# 2. Add the submodule at the new path
git submodule add https://github.com/eddiecarpenter/gh-agentic.git .agents
git -C .agents checkout v2.7.0          # or whatever the current release is

# 3. Update AGENTS.md (if it references @.ai/RULEBOOK.md)
sed -i '' 's|@\.ai/|@.agents/|g' AGENTS.md

# 4. Commit
git add .gitmodules .agents AGENTS.md
git commit -m "chore: rename .ai → .agents (framework dir standard)"
```

If the repo has a `.gitignore` entry for `.ai/`, update that too:

```bash
sed -i '' 's|^\.ai/$|.agents/|' .gitignore
git add .gitignore
```

---

## Repos to update

- [ ] `NewOpenBSS/openbss` (federated control plane)
- [ ] `NewOpenBSS/charging-domain`
- [ ] `eddiecarpenter/ocs-testbench`
- [ ] Any other domain repos mounted via `gh agentic init`

---

## Verification

After the migration, run:

```bash
gh agentic check
```

All checks should pass. If `ai-mounted` fails, ensure `.agents/` is initialised:

```bash
git submodule update --init .agents
```
