#!/usr/bin/env python3
"""check-description-triggers.py — ground-truth check that a skill's
`description:` correctly triggers on the right user phrasings and
correctly does NOT trigger on unrelated ones.

This is the framework's only behavioural quality check. It runs
without Promptfoo: a single Claude API call (via the Claude Code
OAuth credential), JSON parse, ground-truth comparison.

Usage:
    python3 skills/tools/check-description-triggers.py skills/<name>.md

Exit codes:
    0 — all phrasings classified correctly
    1 — one or more mismatches (skill description needs tightening)
    2 — usage error or infrastructure failure (no API access, etc.)

Per-skill ground truth lives in GROUND_TRUTH below. Adding a new
skill: append an entry mapping skill name → {phrase: should_trigger}.
"""

import json
import re
import subprocess
import sys
from pathlib import Path

GROUND_TRUTH: dict[str, dict[str, bool]] = {
    "skill-creator": {
        "Create a skill that posts a comment to a GitHub issue": True,
        "Let's make this reusable": True,
        "Wrap this so we can call it again later": True,
        "Fix the bug in the dev session workflow": False,
        "What's the weather in Cape Town?": False,
    },
    "post-issue-comment": {
        "Publish the design plan on the feature issue": True,
        "Leave a note on the PR": True,
        "Post the iteration summary": True,
        "Apply the in-development label": False,
        "What's the weather in Cape Town?": False,
    },
    "gh-agentic": {
        "What version are we on?": True,
        "What's blocked in the pipeline?": True,
        "List the open features": True,
        "Create a new skill that posts comments": False,
        "What's the weather in Cape Town?": False,
    },
    "requirements-session": {
        "I want to capture a new requirement": True,
        "Let's record this idea as a requirement": True,
        "Add a new business need to the backlog": True,
        "What's blocked in the pipeline?": False,
        "What's the weather in Cape Town?": False,
    },
    "session-init": {
        "Let's start a new session": True,
        "What should we work on?": True,
        "I'm starting a new session": True,
        "Apply the in-development label": False,
        "What's the weather in Cape Town?": False,
    },
    "prompt-user": {
        "Find out from the user which approach they want": True,
        "Confirm with the human before proceeding": True,
        "Let the user decide between these options": True,
        "Read the configuration file": False,
        "What's the weather in Cape Town?": False,
    },
    "apply-label": {
        "Transition the issue to in-development": True,
        "Mark the PR approved": True,
        "Remove the in-design label and add in-development": True,
        "Post the design plan comment": False,
        "What's the weather in Cape Town?": False,
    },
    "set-issue-status": {
        "Transition the requirement to scoping": True,
        "Move the feature to in-design": True,
        "Mark this issue scheduled on the project board": True,
        "Add the in-development label": False,
        "What's the weather in Cape Town?": False,
    },
}

PROMPT_TEMPLATE = (Path(__file__).parent / "check-description-triggers.prompt.txt").read_text()
FENCE = re.compile(r"^\s*```(?:json)?\s*\n?(.*?)\n?```\s*$", re.DOTALL)


def call_claude(prompt: str) -> str:
    """Single Claude call via the local `claude` CLI in non-interactive
    print mode. Uses whichever credentials Claude Code is configured
    with (OAuth subscription or ANTHROPIC_API_KEY)."""
    r = subprocess.run(
        ["claude", "-p"],
        input=prompt, capture_output=True, text=True, check=True,
    )
    return r.stdout


def strip_fence(s: str) -> str:
    m = FENCE.match(s.strip())
    return m.group(1) if m else s


def derive_skill_name(skill_path: Path) -> str:
    """Skill names live in the parent directory under the SKILL.md layout
    (skills/<name>/SKILL.md), so .stem returns "SKILL" — use the parent
    dir name instead. Fall back to .stem for the legacy flat layout."""
    if skill_path.name == "SKILL.md":
        return skill_path.parent.name
    return skill_path.stem


def check(skill_path: Path) -> int:
    name = derive_skill_name(skill_path)
    if name not in GROUND_TRUTH:
        print(f"SKIP {name}: no ground truth defined for this skill", file=sys.stderr)
        return 0
    expected = GROUND_TRUTH[name]
    body = skill_path.read_text()

    prompt = (PROMPT_TEMPLATE
              .replace("{{skill_body}}", body)
              .replace("{{phrasings_json}}", json.dumps(list(expected.keys()), indent=2)))

    raw = call_claude(prompt)
    try:
        got = {p["phrase"]: p["would_trigger"] for p in json.loads(strip_fence(raw))["phrasings"]}
    except (json.JSONDecodeError, KeyError, TypeError) as e:
        print(f"FAIL {name}: malformed model output: {e}", file=sys.stderr)
        print(f"raw output:\n{raw[:500]}", file=sys.stderr)
        return 1

    misses = [(p, expected[p], got.get(p)) for p in expected if expected[p] != got.get(p)]
    if misses:
        print(f"FAIL {name}: {len(misses)}/{len(expected)} mismatches")
        for p, want, g in misses:
            print(f"  {p!r}: expected={want} got={g}")
        return 1
    print(f"PASS {name}: all {len(expected)} phrasings correct")
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(f"usage: {sys.argv[0]} <skill-path>", file=sys.stderr)
        sys.exit(2)
    sys.exit(check(Path(sys.argv[1])))
