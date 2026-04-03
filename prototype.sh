#!/usr/bin/env bash
# gh-agentic bootstrap — UX prototype
# Run this directly in your terminal to feel the flow
# Requires: gum (brew install gum)

GUM=/opt/homebrew/bin/gum

# ── Colours — GitHub palette ──────────────────────────────────────────────────
PRIMARY="#0969DA"    # GitHub blue
SUCCESS="#1A7F37"    # GitHub green
WARNING="#9A6700"    # GitHub yellow/amber
DANGER="#CF222E"     # GitHub red
MUTED="#656D76"      # GitHub muted grey
WHITE="#FFFFFF"
BORDER="#D0D7DE"     # GitHub border grey

# ── Banner ────────────────────────────────────────────────────────────────────
clear
$GUM style \
  --foreground "$WHITE" \
  --border-foreground "$PRIMARY" \
  --border double \
  --align center \
  --width 60 \
  --padding "1 4" \
  "⚡ gh agentic bootstrap" \
  "" \
  "Agentic Development Framework"

echo ""
$GUM style --foreground "$MUTED" "  Initialising new agentic environment..."
echo ""

# ── Preflight ─────────────────────────────────────────────────────────────────
$GUM style --foreground "$PRIMARY" --bold "  Preflight checks"
echo ""

sleep 0.3
$GUM style "  $(
  $GUM style --foreground "$SUCCESS" "✔"
) git found"

sleep 0.2
$GUM style "  $(
  $GUM style --foreground "$SUCCESS" "✔"
) gh found"

sleep 0.2
$GUM style "  $(
  $GUM style --foreground "$SUCCESS" "✔"
) gh authenticated as eddiecarpenter"

sleep 0.2
$GUM style "  $(
  $GUM style --foreground "$SUCCESS" "✔"
) goose found"

sleep 0.2
$GUM style "  $(
  $GUM style --foreground "$WARNING" "⚠"
) claude not found (recommended)"

echo ""
INSTALL_CLAUDE=$($GUM confirm \
  --prompt.foreground "$WARNING" \
  --selected.background "$PRIMARY" \
  --unselected.foreground "$MUTED" \
  "  Install Claude Code now?" && echo "yes" || echo "no")

if [ "$INSTALL_CLAUDE" = "yes" ]; then
  $GUM spin \
    --spinner dot \
    --spinner.foreground "$PRIMARY" \
    --title "  Installing Claude Code..." \
    -- sleep 2
  $GUM style "  $($GUM style --foreground "$SUCCESS" "✔") Claude Code installed"
else
  $GUM style "  $($GUM style --foreground "$MUTED" "·") Skipping Claude Code — continuing without it"
fi

echo ""
$GUM style --foreground "$MUTED" "  ────────────────────────────────────────────────"
echo ""

# ── Topology ──────────────────────────────────────────────────────────────────
$GUM style --foreground "$PRIMARY" --bold "  Step 1 — Topology"
echo ""

TOPOLOGY=$($GUM choose \
  --header "  How is this project structured?" \
  --header.foreground "$MUTED" \
  --cursor.foreground "$PRIMARY" \
  --selected.foreground "$WHITE" \
  --height 4 \
  "Embedded   — single repo, all-in-one" \
  "Organisation — separate agentic control plane")

echo ""
$GUM style "  $(
  $GUM style --foreground "$SUCCESS" "✔"
) Topology: $($GUM style --foreground "$WHITE" --bold "${TOPOLOGY%%—*}")"

echo ""
$GUM style --foreground "$MUTED" "  ────────────────────────────────────────────────"
echo ""

# ── Owner ─────────────────────────────────────────────────────────────────────
$GUM style --foreground "$PRIMARY" --bold "  Step 2 — Owner"
echo ""

OWNER=$($GUM choose \
  --header "  Where should the repo be created?" \
  --header.foreground "$MUTED" \
  --cursor.foreground "$PRIMARY" \
  --selected.foreground "$WHITE" \
  --height 6 \
  "eddiecarpenter       (personal)" \
  "NewOpenBSS           ✔ clean" \
  "open-bss             ✔ clean" \
  "quarkiverse          ⚠ has repos")

echo ""
$GUM style "  $(
  $GUM style --foreground "$SUCCESS" "✔"
) Owner: $($GUM style --foreground "$WHITE" --bold "${OWNER%%[[:space:]]*(*)}")"

echo ""
$GUM style --foreground "$MUTED" "  ────────────────────────────────────────────────"
echo ""

# ── Project details ───────────────────────────────────────────────────────────
$GUM style --foreground "$PRIMARY" --bold "  Step 3 — Project details"
echo ""

PROJECT_NAME=$($GUM input \
  --placeholder "my-project" \
  --prompt "  Name › " \
  --prompt.foreground "$PRIMARY" \
  --width 40)

DESCRIPTION=$($GUM input \
  --placeholder "What does this project do?" \
  --prompt "  Description › " \
  --prompt.foreground "$PRIMARY" \
  --width 60)

echo ""
STACK=$($GUM choose \
  --header "  Stack" \
  --header.foreground "$MUTED" \
  --cursor.foreground "$PRIMARY" \
  --selected.foreground "$WHITE" \
  --height 8 \
  "Go" \
  "Java — Quarkus" \
  "Java — Spring Boot" \
  "TypeScript / Node.js" \
  "Python" \
  "Rust" \
  "Other")

echo ""
ANTORA=$($GUM confirm \
  --prompt.foreground "$PRIMARY" \
  --selected.background "$PRIMARY" \
  --unselected.foreground "$MUTED" \
  "  Antora documentation site?" && echo "Yes" || echo "No")

echo ""
$GUM style --foreground "$MUTED" "  ────────────────────────────────────────────────"
echo ""

# ── Summary ───────────────────────────────────────────────────────────────────
$GUM style --foreground "$PRIMARY" --bold "  Summary"
echo ""

$GUM style \
  --border rounded \
  --border-foreground "$PRIMARY" \
  --padding "1 3" \
  --width 56 \
  "$($GUM style --foreground "$MUTED" "Topology   ") $($GUM style --foreground "$WHITE" "${TOPOLOGY%%—*}")" \
  "$($GUM style --foreground "$MUTED" "Owner      ") $($GUM style --foreground "$WHITE" "${OWNER%%[[:space:]]*(*)}")" \
  "$($GUM style --foreground "$MUTED" "Name       ") $($GUM style --foreground "$WHITE" "$PROJECT_NAME")" \
  "$($GUM style --foreground "$MUTED" "Description") $($GUM style --foreground "$WHITE" "$DESCRIPTION")" \
  "$($GUM style --foreground "$MUTED" "Stack      ") $($GUM style --foreground "$WHITE" "$STACK")" \
  "$($GUM style --foreground "$MUTED" "Antora     ") $($GUM style --foreground "$WHITE" "$ANTORA")"

echo ""

$GUM confirm \
  --prompt.foreground "$WHITE" \
  --selected.background "$PRIMARY" \
  --unselected.foreground "$MUTED" \
  "  Create project?" || { echo ""; $GUM style --foreground "$MUTED" "  Aborted."; echo ""; exit 0; }

echo ""
$GUM style --foreground "$MUTED" "  ────────────────────────────────────────────────"
echo ""

# ── Execution ─────────────────────────────────────────────────────────────────
$GUM style --foreground "$PRIMARY" --bold "  Creating your agentic environment"
echo ""

steps=(
  "Creating repository"
  "Removing template files"
  "Scaffolding $STACK project"
  "Configuring labels"
  "Populating repository"
  "Creating GitHub Project"
)

for step in "${steps[@]}"; do
  $GUM spin \
    --spinner dot \
    --spinner.foreground "$PRIMARY" \
    --title "  $step..." \
    -- sleep 1
  $GUM style "  $($GUM style --foreground "$SUCCESS" "✔") $step"
done

echo ""
$GUM style --foreground "$MUTED" "  ────────────────────────────────────────────────"
echo ""

# ── Final summary ─────────────────────────────────────────────────────────────
$GUM style --foreground "$SUCCESS" --bold "  ✔ Bootstrap complete"
echo ""

$GUM style \
  --border rounded \
  --border-foreground "$SUCCESS" \
  --padding "1 3" \
  --width 56 \
  "$($GUM style --foreground "$MUTED" "Repo   ") $($GUM style --foreground "$PRIMARY" "https://github.com/${OWNER%%[[:space:]]*(*)}/$(echo $PROJECT_NAME | tr '[:upper:]' '[:lower:]')")" \
  "$($GUM style --foreground "$MUTED" "Project") $($GUM style --foreground "$PRIMARY" "https://github.com/orgs/${OWNER%%[[:space:]]*(*)}/projects/1")" \
  "$($GUM style --foreground "$MUTED" "Clone  ") $($GUM style --foreground "$WHITE" "~/Development/$PROJECT_NAME")"

echo ""
$GUM style --foreground "$MUTED" "  ────────────────────────────────────────────────"
echo ""

# ── Launch Goose ──────────────────────────────────────────────────────────────
CLONE_PATH=~/Development/$PROJECT_NAME

$GUM style --foreground "$PRIMARY" --bold "  Start Requirements Session"
echo ""
$GUM style --foreground "$MUTED" "  How would you like to continue?"
echo ""

LAUNCH=$($GUM choose \
  --cursor.foreground "$PRIMARY" \
  --selected.foreground "$WHITE" \
  --height 3 \
  "Terminal  — launch Goose CLI in $PROJECT_NAME" \
  "Skip      — I'll do it manually")

echo ""

case "$LAUNCH" in
  Terminal*)
    $GUM spin \
      --spinner dot \
      --spinner.foreground "$PRIMARY" \
      --title "  Launching Goose..." \
      -- sleep 1
    $GUM style "  $($GUM style --foreground "$SUCCESS" "✔") Goose launched in $CLONE_PATH"
    echo ""
    $GUM style --foreground "$MUTED" "  Your agent will read context and begin the Requirements Session."
    echo ""
    # cd "$CLONE_PATH" && goose session --recipe requirements
    ;;
  Skip*)
    $GUM style --foreground "$MUTED" "  When ready, open Goose in:"
    $GUM style --foreground "$WHITE"  "  $CLONE_PATH"
    echo ""
    ;;
esac

echo ""
