#!/usr/bin/env bash
# verify-framework-content.sh — Verify that the framework content migrated from
# ai-native-delivery is complete and references are updated in gh-agentic.
#
# Exit 0 if all checks pass, non-zero otherwise.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ERRORS=0

# --- Colour helpers (no-op if not a terminal) ---
if [ -t 1 ]; then
  RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
else
  RED=''; GREEN=''; YELLOW=''; NC=''
fi

pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; ERRORS=$((ERRORS + 1)); }
warn() { echo -e "${YELLOW}⚠${NC} $1"; }

# ============================================================================
# 1. Framework core content — RULEBOOK.md, skills/, standards/, concepts/
# ============================================================================
echo ""
echo "=== Framework Core Content ==="

# RULEBOOK.md
[ -f "$REPO_ROOT/RULEBOOK.md" ] && pass "RULEBOOK.md exists" || fail "RULEBOOK.md missing"

# Skills (14 files)
EXPECTED_SKILLS=(
  capture-feature.md
  dev-session.md
  feature-design.md
  feature-scoping.md
  foreground-recovery.md
  issue-session.md
  notify-user.md
  post-sync.md
  pr-review-session.md
  release-notes.md
  requirements-session.md
  session-init.md
  set-issue-status.md
  update-project-template.md
)

for skill in "${EXPECTED_SKILLS[@]}"; do
  [ -f "$REPO_ROOT/skills/$skill" ] && pass "skills/$skill" || fail "skills/$skill missing"
done

# Standards
[ -f "$REPO_ROOT/standards/go.md" ] && pass "standards/go.md" || fail "standards/go.md missing"

# Concepts
[ -f "$REPO_ROOT/concepts/delivery-philosophy.md" ] && pass "concepts/delivery-philosophy.md" || fail "concepts/delivery-philosophy.md missing"
[ -f "$REPO_ROOT/concepts/feature-switches.md" ] && pass "concepts/feature-switches.md" || fail "concepts/feature-switches.md missing"

# ============================================================================
# 2. Recipes (8 files)
# ============================================================================
echo ""
echo "=== Recipes ==="

EXPECTED_RECIPES=(
  dev-session.yaml
  feature-design.yaml
  feature-scoping.yaml
  foreground-recovery.yaml
  issue-session.yaml
  pr-review-session.yaml
  release.yaml
  requirements-session.yaml
)

for recipe in "${EXPECTED_RECIPES[@]}"; do
  [ -f "$REPO_ROOT/recipes/$recipe" ] && pass "recipes/$recipe" || fail "recipes/$recipe missing"
done

# ============================================================================
# 3. GitHub workflows and actions
# ============================================================================
echo ""
echo "=== GitHub Workflows ==="

EXPECTED_WORKFLOWS=(
  add-issue-to-project.yml
  agentic-pipeline.yml
  publish-release.yml
  release.yml
  sonarcloud.yml
)

for wf in "${EXPECTED_WORKFLOWS[@]}"; do
  [ -f "$REPO_ROOT/.github/workflows/$wf" ] && pass ".github/workflows/$wf" || fail ".github/workflows/$wf missing"
done

echo ""
echo "=== GitHub Composite Actions ==="

EXPECTED_ACTIONS=(
  install-ai-tools/action.yml
  install-system-deps/action.yml
  setup-claude-auth/action.yml
)

for action in "${EXPECTED_ACTIONS[@]}"; do
  [ -f "$REPO_ROOT/.github/actions/$action" ] && pass ".github/actions/$action" || fail ".github/actions/$action missing"
done

# ============================================================================
# 4. Cross-reference check — no stale ai-native-delivery references
# ============================================================================
echo ""
echo "=== Cross-Reference Check ==="

# Scan only the framework content directories (not .ai/, LOCALRULES.md, AGENTS.md)
STALE_REFS=$(grep -r "ai-native-delivery" \
  "$REPO_ROOT/RULEBOOK.md" \
  "$REPO_ROOT/skills/" \
  "$REPO_ROOT/recipes/" \
  "$REPO_ROOT/standards/" \
  "$REPO_ROOT/concepts/" \
  "$REPO_ROOT/.github/workflows/" \
  "$REPO_ROOT/.github/actions/" \
  2>/dev/null || true)

if [ -z "$STALE_REFS" ]; then
  pass "No stale ai-native-delivery references in framework content"
else
  fail "Stale ai-native-delivery references found:"
  echo "$STALE_REFS" | while IFS= read -r line; do
    echo "    $line"
  done
fi

# ============================================================================
# 5. gh-agentic-specific workflows preserved
# ============================================================================
echo ""
echo "=== gh-agentic Specific Workflows ==="

[ -f "$REPO_ROOT/.github/workflows/build-and-test.yml" ] && pass "build-and-test.yml preserved" || warn "build-and-test.yml not found (gh-agentic specific)"
[ -f "$REPO_ROOT/.github/workflows/agentic-pipeline-reusable.yml" ] && pass "agentic-pipeline-reusable.yml preserved" || warn "agentic-pipeline-reusable.yml not found (gh-agentic specific)"

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "==============================="
if [ "$ERRORS" -eq 0 ]; then
  echo -e "${GREEN}All checks passed.${NC}"
  exit 0
else
  echo -e "${RED}${ERRORS} check(s) failed.${NC}"
  exit 1
fi
