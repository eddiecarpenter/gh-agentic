#!/usr/bin/env bash
# test-cp-layout.sh — Layer 2 structure check for the control-plane-centralized
# pipeline (#873).
#
# Reproduces the agentic-pipeline's CP-rooted / ./project layout with real LOCAL
# clones — no network, no GitHub writes, no goose/agent — then asserts every
# filesystem invariant the recipe and execution skills depend on. This is the
# check that catches layout regressions (e.g. .agents resolving to the wrong
# root) without a full end-to-end pipeline run.
#
# It mirrors the workflow's execution-job steps:
#   1. Checkout knowledge root (CP @ main)   -> local clone of this repo
#   2. Resolve target repo                    -> single: the CP itself
#                                                federated: a scratch pure-code repo
#   3. Checkout project (code) into ./project -> clone of the target
#   4. Isolate project + export anchors       -> the REAL run-block, verbatim
# then checks the resulting tree.
#
# Usage:  scripts/test-cp-layout.sh
# Exit:   0 = all invariants hold; non-zero = a layout invariant failed.
set -u

# Repo root = the git toplevel containing this script.
SRC="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null)"
if [ -z "${SRC}" ] || [ ! -e "${SRC}/.github/workflows/agentic-pipeline.yml" ]; then
  echo "::error::could not locate the gh-agentic repo root from $(dirname "$0")"
  exit 2
fi

PASS=0; FAIL=0
ok()  { printf '  \033[32m✓\033[0m %s\n' "$1"; PASS=$((PASS+1)); }
bad() { printf '  \033[31m✗ %s\033[0m\n' "$1"; FAIL=$((FAIL+1)); }
chk() { if eval "$2"; then ok "$1"; else bad "$1  [cmd: $2]"; fi; }

# The exact "Isolate project and export anchors" run-block from agentic-pipeline.yml.
# Kept verbatim so this check fails if the workflow's anchor contract drifts.
isolate_and_anchors() {
  local WS="$1" ENVFILE="$2"
  ( cd "$WS" || exit 1
    GITHUB_WORKSPACE="$WS"
    GITHUB_ENV="$ENVFILE"
    [ -d .git ] && echo '/project/' >> .git/info/exclude
    echo "AGENTIC_CP_ROOT=${GITHUB_WORKSPACE}" >> "$GITHUB_ENV"
    echo "AGENTIC_PROJECT_DIR=${GITHUB_WORKSPACE}/project" >> "$GITHUB_ENV"
  )
}

run_case() {
  local NAME="$1" TARGET_SRC="$2" REF="$3"
  echo ""
  echo "=== Case: ${NAME} ==="
  local ROOT; ROOT="$(mktemp -d)"
  local WS="${ROOT}/ws"

  # Step 1 — Checkout knowledge root (CP @ main) at the workspace root.
  git clone --quiet "file://${SRC}" "${WS}" 2>/dev/null
  git -C "${WS}" checkout --quiet main 2>/dev/null || true

  # Step 3 — Checkout project (code) into ./project at the per-job ref.
  git clone --quiet "file://${TARGET_SRC}" "${WS}/project" 2>/dev/null
  [ -n "${REF}" ] && git -C "${WS}/project" checkout --quiet "${REF}" 2>/dev/null || true

  # Step 4 — Isolate + export anchors (the real run-block).
  local ENVFILE="${ROOT}/gh_env"; : > "${ENVFILE}"
  isolate_and_anchors "${WS}" "${ENVFILE}"

  # Load the exported anchors as the recipe step would see them.
  # shellcheck disable=SC1090
  set -a; . "${ENVFILE}"; set +a
  echo "  AGENTIC_CP_ROOT=${AGENTIC_CP_ROOT}"
  echo "  AGENTIC_PROJECT_DIR=${AGENTIC_PROJECT_DIR}"

  # --- Invariants the recipe + skills depend on ---
  chk "recipe resolves: \$AGENTIC_CP_ROOT/.agents/.goose/recipes/feature-design.yaml" \
      "[ -f \"${AGENTIC_CP_ROOT}/.agents/.goose/recipes/feature-design.yaml\" ]"
  chk "rulebook resolves: \$AGENTIC_CP_ROOT/AGENTS.md" \
      "[ -f \"${AGENTIC_CP_ROOT}/AGENTS.md\" ]"
  chk "docs resolve: \$AGENTIC_CP_ROOT/docs/ARCHITECTURE.md" \
      "[ -f \"${AGENTIC_CP_ROOT}/docs/ARCHITECTURE.md\" ]"
  chk "code present at \$AGENTIC_PROJECT_DIR (.git)" \
      "[ -d \"${AGENTIC_PROJECT_DIR}/.git\" ]"
  chk "/project/ in CP .git/info/exclude" \
      "grep -qx '/project/' \"${AGENTIC_CP_ROOT}/.git/info/exclude\""
  chk "CP working tree clean (./project invisible to it)" \
      "[ -z \"\$(git -C \"${AGENTIC_CP_ROOT}\" status --porcelain)\" ]"
  chk "composite action resolves: \$AGENTIC_CP_ROOT/.agents/.github/actions/setup-goose-env" \
      "[ -d \"${AGENTIC_CP_ROOT}/.agents/.github/actions/setup-goose-env\" ]"

  rm -rf "${ROOT}"
}

# --- Build a scratch pure-code domain repo as a federated target ---
SCRATCH_PARENT="$(mktemp -d)"
SCRATCH="${SCRATCH_PARENT}/domain-repo"
mkdir -p "${SCRATCH}"
git -C "${SCRATCH}" init --quiet
git -C "${SCRATCH}" checkout --quiet -b main
printf 'package main\nfunc main(){}\n' > "${SCRATCH}/main.go"
git -C "${SCRATCH}" -c user.email=t@t -c user.name=t add -A
git -C "${SCRATCH}" -c user.email=t@t -c user.name=t commit --quiet -m "code"
git -C "${SCRATCH}" checkout --quiet -b feature/42-add-login
printf '// feature work\n' >> "${SCRATCH}/main.go"
git -C "${SCRATCH}" -c user.email=t@t -c user.name=t commit --quiet -am "wip"

echo "=== scratch domain repo (must be pure code) ==="
chk "domain repo has NO .agents" "[ ! -e \"${SCRATCH}/.agents\" ]"
chk "domain repo has NO docs/"   "[ ! -e \"${SCRATCH}/docs\" ]"

# Case 1: single-topology — target == CP (feature-design checks out the target @ main).
run_case "single-topology (target = CP itself)" "${SRC}" "main"

# Case 2: federated — target is a pure-code domain repo @ the feature branch.
run_case "federated (target = pure-code domain repo @ feature branch)" "${SCRATCH}" "feature/42-add-login"

rm -rf "${SCRATCH_PARENT}"

echo ""
echo "=================================================="
printf "Layer 2 structure check: \033[32m%d passed\033[0m, " "${PASS}"
if [ "${FAIL}" -gt 0 ]; then printf "\033[31m%d FAILED\033[0m\n" "${FAIL}"; else printf "%d failed\n" "${FAIL}"; fi
echo "=================================================="
[ "${FAIL}" -eq 0 ]
