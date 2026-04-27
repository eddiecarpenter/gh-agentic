#!/usr/bin/env python3
"""Mechanical verification of skill files against the skill-spec.

Runs deterministic checks against a skill's structure and frontmatter.
Does not assess behavioural quality — that is the job of the
ground-truth check scripts (e.g., check-description-triggers.py).

Usage:
    python skills/tools/verify-skill-mechanical.py <skill-path>
    python skills/tools/verify-skill-mechanical.py <skill-path> --check <name>
    python skills/tools/verify-skill-mechanical.py <skill-path> --format json|human

Exit codes:
    0 - all checks passed
    1 - one or more checks failed
    2 - usage error or unreadable skill file
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable

# ---------------------------------------------------------------------------
# Spec constants — single source of truth for what the spec requires.
# Update these if the spec's requirements change.
# ---------------------------------------------------------------------------

MANDATORY_SECTIONS = [
    "Goal",
    "Output Artefacts",
    "Definitions",
    "Dependencies",
    "Steps",
    "Verification",
    "Error Handling",
]

REQUIRED_FRONTMATTER_FIELDS = ["name", "description", "triggers"]

NAME_PATTERN = re.compile(r"^[a-z0-9-]{1,64}$")

DESCRIPTION_MAX_CHARS = 1024

# Phrases that suggest non-third-person voice in description.
NON_THIRD_PERSON_HINTS = [
    "i can ",
    "i will ",
    "i help ",
    "you can ",
    "you should ",
    "you may ",
    "your skill ",
]

# Phrases that suggest the description has the assertive "use when" clause.
ASSERTIVE_HINTS = ["use when", "use this when"]

# ---------------------------------------------------------------------------
# Skill model
# ---------------------------------------------------------------------------

@dataclass
class Skill:
    """Parsed skill: frontmatter (as raw text + parsed dict) and body."""
    path: Path
    raw: str
    frontmatter_text: str
    frontmatter: dict
    body: str

    @classmethod
    def load(cls, path: Path) -> "Skill":
        raw = path.read_text()
        fm_text, body = _split_frontmatter(raw)
        fm = _parse_frontmatter(fm_text) if fm_text else {}
        return cls(path=path, raw=raw, frontmatter_text=fm_text, frontmatter=fm, body=body)


def _split_frontmatter(raw: str) -> tuple[str, str]:
    """Extract YAML frontmatter from a markdown file.

    Returns (frontmatter_text, body_text). Frontmatter is empty string
    if the file does not start with a `---` block.
    """
    if not raw.startswith("---\n"):
        return "", raw
    end = raw.find("\n---\n", 4)
    if end == -1:
        return "", raw
    return raw[4:end], raw[end + 5:]


def _parse_frontmatter(text: str) -> dict:
    """Minimal YAML frontmatter parser.

    Handles only the subset our skill spec requires: scalar string values
    (key: value) and one-level lists (key:\n  - item\n  - item). Quoted
    strings are unquoted. Trailing colons inside values are preserved.

    Not a general YAML parser. Sufficient for skill frontmatter only.
    """
    out: dict = {}
    current_list_key: str | None = None
    for line in text.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        # Continuation of a list under the previous key
        if current_list_key and line.startswith(("  -", "    -", "\t-")):
            item = line.lstrip().lstrip("-").strip()
            out[current_list_key].append(_unquote(item))
            continue
        # New key
        if ":" in line and not line.startswith((" ", "\t")):
            current_list_key = None
            key, _, value = line.partition(":")
            key = key.strip()
            value = value.strip()
            if value == "":
                # Likely the start of a list
                out[key] = []
                current_list_key = key
            else:
                out[key] = _unquote(value)
            continue
        # Anything else — ignore (multi-line scalars not supported here)
    return out


def _unquote(s: str) -> str:
    if len(s) >= 2 and s[0] == s[-1] and s[0] in ("'", '"'):
        return s[1:-1]
    return s


# ---------------------------------------------------------------------------
# Check result model
# ---------------------------------------------------------------------------

@dataclass
class CheckResult:
    name: str
    passed: bool
    detail: str = ""
    extra: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        d = {"pass": self.passed}
        if self.detail:
            d["detail"] = self.detail
        if self.extra:
            d.update(self.extra)
        return d


# ---------------------------------------------------------------------------
# Individual checks. Each function takes the loaded Skill and returns
# a CheckResult. Add new checks here; register them in CHECKS below.
# ---------------------------------------------------------------------------

def check_all_sections_present(skill: Skill) -> CheckResult:
    """Every mandatory section heading is present in the body."""
    missing = [s for s in MANDATORY_SECTIONS if f"## {s}" not in skill.body]
    if missing:
        return CheckResult(
            "all_sections_present",
            passed=False,
            detail=f"Missing section heading(s): {', '.join(missing)}",
            extra={"missing": missing},
        )
    return CheckResult("all_sections_present", passed=True)


def check_frontmatter_required_fields(skill: Skill) -> CheckResult:
    """Required frontmatter fields are declared and non-empty."""
    missing = []
    empty = []
    for f in REQUIRED_FRONTMATTER_FIELDS:
        if f not in skill.frontmatter:
            missing.append(f)
        elif not skill.frontmatter[f]:
            empty.append(f)
    if missing or empty:
        details = []
        if missing:
            details.append(f"missing: {', '.join(missing)}")
        if empty:
            details.append(f"empty: {', '.join(empty)}")
        return CheckResult(
            "frontmatter_required_fields",
            passed=False,
            detail="; ".join(details),
            extra={"missing": missing, "empty": empty},
        )
    return CheckResult("frontmatter_required_fields", passed=True)


def check_frontmatter_name_valid(skill: Skill) -> CheckResult:
    """Name is kebab-case, lowercase letters/digits/hyphens, ≤64 chars."""
    name = skill.frontmatter.get("name", "")
    if not name:
        return CheckResult(
            "frontmatter_name_valid",
            passed=False,
            detail="name is missing or empty",
        )
    if not NAME_PATTERN.match(name):
        return CheckResult(
            "frontmatter_name_valid",
            passed=False,
            detail=f"name {name!r} does not match kebab-case pattern (lowercase letters, digits, hyphens; ≤64 chars)",
        )
    return CheckResult("frontmatter_name_valid", passed=True)


def check_description_within_length_limit(skill: Skill) -> CheckResult:
    """Description is within Anthropic's 1024-char hard limit."""
    desc = skill.frontmatter.get("description", "")
    length = len(desc)
    if length > DESCRIPTION_MAX_CHARS:
        return CheckResult(
            "description_within_length_limit",
            passed=False,
            detail=f"description is {length} chars; limit is {DESCRIPTION_MAX_CHARS}",
            extra={"length": length, "limit": DESCRIPTION_MAX_CHARS},
        )
    return CheckResult(
        "description_within_length_limit",
        passed=True,
        detail=f"{length}/{DESCRIPTION_MAX_CHARS} chars",
        extra={"length": length},
    )


def check_description_assertive(skill: Skill) -> CheckResult:
    """Description contains an assertive 'use when' clause."""
    desc_lower = skill.frontmatter.get("description", "").lower()
    matched = [hint for hint in ASSERTIVE_HINTS if hint in desc_lower]
    if not matched:
        return CheckResult(
            "description_assertive",
            passed=False,
            detail="description does not contain an assertive 'Use when' clause",
        )
    return CheckResult("description_assertive", passed=True, detail=f"matched: {matched[0]!r}")


def check_description_third_person(skill: Skill) -> CheckResult:
    """Description does not use first or second person voice (heuristic)."""
    desc_lower = skill.frontmatter.get("description", "").lower()
    hits = [h for h in NON_THIRD_PERSON_HINTS if h in desc_lower]
    if hits:
        return CheckResult(
            "description_third_person",
            passed=False,
            detail=f"description appears to use non-third-person voice; matched: {hits}",
            extra={"matches": hits},
        )
    return CheckResult("description_third_person", passed=True)


def check_references_resolve(skill: Skill) -> CheckResult:
    """Paths declared in the `loads:` frontmatter resolve to existing files.

    The frontmatter `loads:` list is the canonical, machine-readable
    declaration of every Definition and Dependency the skill consults
    or invokes. Body-section mentions of paths (for illustration,
    forward-references, examples) are documentation only and are not
    checked here — the spec requires `loads:` to mirror Definitions +
    Dependencies, so the frontmatter is the source of truth.
    """
    repo_root = _find_repo_root(skill.path)
    referenced = list(skill.frontmatter.get("loads") or [])
    if not referenced:
        return CheckResult("references_resolve", passed=True, detail="no references")
    missing = []
    for ref in referenced:
        if not (repo_root / ref).exists():
            missing.append(ref)
    if missing:
        return CheckResult(
            "references_resolve",
            passed=False,
            detail=f"missing reference(s): {', '.join(missing)}",
            extra={"missing": missing, "checked": list(referenced)},
        )
    return CheckResult(
        "references_resolve",
        passed=True,
        detail=f"all {len(referenced)} reference(s) resolve",
    )


def _find_repo_root(skill_path: Path) -> Path:
    """Walk up from the skill path to find a directory with skills/ in it."""
    p = skill_path.resolve().parent
    while p != p.parent:
        if (p / "skills").is_dir() and (p / ".git").exists():
            return p
        p = p.parent
    # Fallback: assume the skill is two levels deep from repo root
    return skill_path.resolve().parent.parent


def _extract_referenced_paths(body: str) -> set[str]:
    """Extract paths in the Definitions and Dependencies sections.

    Looks for strings matching a skills/ or skills/definitions/ markdown path
    inside the relevant sections. Heuristic — matches paths in backticks or
    bare paths in list items.
    """
    paths: set[str] = set()
    in_section = False
    for line in body.splitlines():
        stripped = line.strip()
        if stripped.startswith("## Definitions") or stripped.startswith("## Dependencies"):
            in_section = True
            continue
        if stripped.startswith("## ") and in_section:
            in_section = False
        if not in_section:
            continue
        # Find any `path/like/this.md` references
        for match in re.finditer(r"`([^`]+\.md)`", line):
            paths.add(match.group(1))
        for match in re.finditer(r"(?:^|\s)((?:concepts|skills|docs|tools)/[A-Za-z0-9_./-]+\.md)", line):
            paths.add(match.group(1))
    return paths


# A skill loading a skills/definitions/*.md file MUST NOT also restate
# the definition's content inline — the load directive is the canonical
# reference. The check below catches drift where a maintainer adds a
# load directive but forgets to remove the now-duplicate inline prose.
DUPLICATION_RUN_LENGTH = 4
_TRIVIAL_LINE_RE = re.compile(r"^(?:\s*|\s*```.*|\s*#+\s*.*|\s*[-*]\s*|\s*\|.*)$")


def _meaningful_lines(text: str) -> list[tuple[int, str]]:
    """Return (1-based line number, stripped content) for non-trivial lines.

    Trivial lines (blank, fence markers, headings, list-bullet skeletons,
    table separators) are excluded so a run of N consecutive meaningful
    lines is a strong duplication signal rather than coincidental
    structural overlap.
    """
    out: list[tuple[int, str]] = []
    for i, line in enumerate(text.splitlines(), start=1):
        stripped = line.strip()
        if not stripped or _TRIVIAL_LINE_RE.match(line):
            continue
        # Drop very short lines (under 20 chars) — phrase-level overlap
        # is not duplication of substance.
        if len(stripped) < 20:
            continue
        out.append((i, stripped))
    return out


def check_no_duplicated_definition_content(skill: Skill) -> CheckResult:
    """A skill loading a definition MUST NOT restate its content inline.

    For each `skills/definitions/*.md` entry in the skill's `loads:`
    frontmatter, look for a run of `DUPLICATION_RUN_LENGTH` consecutive
    meaningful lines from the definition appearing verbatim in the skill
    body. Failure surfaces the skill, the definition, and the line
    range of the first offending run.
    """
    loads = skill.frontmatter.get("loads", [])
    if not isinstance(loads, list):
        return CheckResult("no_duplicated_definition_content", passed=True,
                           detail="no loads list to check")

    repo_root = _find_repo_root(skill.path)
    skill_body_set = set(line for _, line in _meaningful_lines(skill.body))
    skill_body_lines = [line for _, line in _meaningful_lines(skill.body)]

    duplications: list[dict] = []
    for ref in loads:
        if not ref.startswith("skills/definitions/"):
            continue
        def_path = repo_root / ref
        if not def_path.exists():
            continue  # references_resolve catches this separately
        def_lines = _meaningful_lines(def_path.read_text())
        # Slide a window of DUPLICATION_RUN_LENGTH over the definition's
        # meaningful lines; flag if every line in the window also appears
        # in the skill body.
        for i in range(len(def_lines) - DUPLICATION_RUN_LENGTH + 1):
            window = def_lines[i:i + DUPLICATION_RUN_LENGTH]
            if all(line in skill_body_set for _, line in window):
                # Now require the matching skill-body lines to be
                # contiguous as well — otherwise it's coincidental
                # phrase reuse, not block duplication.
                first_def_line = window[0][1]
                if first_def_line not in skill_body_lines:
                    continue
                start_idx = skill_body_lines.index(first_def_line)
                contiguous = all(
                    start_idx + k < len(skill_body_lines)
                    and skill_body_lines[start_idx + k] == window[k][1]
                    for k in range(DUPLICATION_RUN_LENGTH)
                )
                if contiguous:
                    duplications.append({
                        "definition": ref,
                        "definition_lines": f"{window[0][0]}-{window[-1][0]}",
                        "first_match": first_def_line[:80],
                    })
                    break  # one report per definition is enough

    if duplications:
        first = duplications[0]
        detail = (
            f"skill body restates {len(duplications)} loaded definition(s); "
            f"first: {first['definition']} lines {first['definition_lines']}"
        )
        return CheckResult(
            "no_duplicated_definition_content",
            passed=False,
            detail=detail,
            extra={"duplications": duplications},
        )
    return CheckResult("no_duplicated_definition_content", passed=True)


# ---------------------------------------------------------------------------
# Check registry
# ---------------------------------------------------------------------------

CHECKS: dict[str, Callable[[Skill], CheckResult]] = {
    "all_sections_present": check_all_sections_present,
    "frontmatter_required_fields": check_frontmatter_required_fields,
    "frontmatter_name_valid": check_frontmatter_name_valid,
    "description_within_length_limit": check_description_within_length_limit,
    "description_assertive": check_description_assertive,
    "description_third_person": check_description_third_person,
    "references_resolve": check_references_resolve,
    "no_duplicated_definition_content": check_no_duplicated_definition_content,
}


# ---------------------------------------------------------------------------
# Output formatting
# ---------------------------------------------------------------------------

def format_json(skill_path: Path, results: list[CheckResult]) -> str:
    payload = {
        "skill": str(skill_path),
        "checks": {r.name: r.to_dict() for r in results},
        "summary": {
            "passed": sum(1 for r in results if r.passed),
            "failed": sum(1 for r in results if not r.passed),
            "total": len(results),
        },
    }
    return json.dumps(payload, indent=2)


def format_human(skill_path: Path, results: list[CheckResult]) -> str:
    lines = [f"Skill: {skill_path}", ""]
    width = max(len(r.name) for r in results) if results else 0
    for r in results:
        status = "✓" if r.passed else "✗"
        line = f"  {status} {r.name.ljust(width)}"
        if r.detail:
            line += f"  — {r.detail}"
        lines.append(line)
    passed = sum(1 for r in results if r.passed)
    failed = len(results) - passed
    lines.extend(["", f"Summary: {passed} passed, {failed} failed (out of {len(results)})"])
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main() -> int:
    p = argparse.ArgumentParser(
        description="Mechanical verification of skill files against the skill-spec."
    )
    p.add_argument("skill_path", type=Path, help="Path to the skill file to verify.")
    p.add_argument(
        "--check",
        action="append",
        choices=sorted(CHECKS.keys()),
        help="Run only the named check(s). Repeat to run several. Default: all.",
    )
    p.add_argument(
        "--format",
        choices=["json", "human"],
        default="human",
        help="Output format. Default: human.",
    )
    args = p.parse_args()

    if not args.skill_path.is_file():
        print(f"error: skill file not found: {args.skill_path}", file=sys.stderr)
        return 2

    try:
        skill = Skill.load(args.skill_path)
    except Exception as e:
        print(f"error: failed to parse skill: {e}", file=sys.stderr)
        return 2

    selected = args.check if args.check else list(CHECKS.keys())
    results = [CHECKS[name](skill) for name in selected]

    if args.format == "json":
        print(format_json(args.skill_path, results))
    else:
        print(format_human(args.skill_path, results))

    return 0 if all(r.passed for r in results) else 1


if __name__ == "__main__":
    sys.exit(main())
