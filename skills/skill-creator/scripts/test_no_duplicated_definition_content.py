"""Tests for the no_duplicated_definition_content check.

Run with: python3 -m unittest skills/skill-creator/scripts/test_no_duplicated_definition_content.py
"""
from pathlib import Path
import shutil
import sys
import tempfile
import textwrap
import unittest

sys.path.insert(0, str(Path(__file__).parent))
from importlib import import_module
verify_mod = import_module("verify-skill-mechanical")


SKILL_TEMPLATE = textwrap.dedent("""\
    ---
    name: {name}
    description: Test skill. Use when testing the duplication check on a small fixture skill body.
    triggers: human
    loads:
      - skills/definitions/test-definition.md
    ---

    # Test Skill

    ## Goal
    Test goal.

    ## Output Artefacts
    Test artefacts.

    ## Definitions
    None.

    ## Dependencies
    None.

    ## Steps
    {steps_body}

    ## Verification
    Per skills/definitions/verification-procedure.md.

    ## Error Handling
    None.
    """)


DEFINITION_BODY = textwrap.dedent("""\
    # Test Definition

    The skill must apply the canonical safety guard before mutating state.
    Every consumer skill is required to follow the safe-default protocol.
    Failures must surface a warning to the operator and halt the work.
    On retry the skill should re-check whether the precondition still holds.
    Documentation should record the outcome with a stable identifier.
    """)


class TestNoDuplicatedDefinitionContent(unittest.TestCase):

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp())
        # Build a minimal repo layout — .git marker is required for
        # _find_repo_root to terminate at our temp root.
        (self.tmp / ".git").mkdir()
        (self.tmp / "skills" / "definitions").mkdir(parents=True)
        (self.tmp / "skills" / "test-skill").mkdir(parents=True)
        # Write definition
        (self.tmp / "skills" / "definitions" / "test-definition.md").write_text(
            DEFINITION_BODY
        )

    def tearDown(self):
        shutil.rmtree(self.tmp)

    def _run(self, steps_body: str) -> verify_mod.CheckResult:
        skill_path = self.tmp / "skills" / "test-skill" / "SKILL.md"
        skill_path.write_text(SKILL_TEMPLATE.format(
            name="test-skill", steps_body=steps_body))
        skill = verify_mod.Skill.load(skill_path)
        return verify_mod.check_no_duplicated_definition_content(skill)

    def test_clean_skill_passes(self):
        """Skill loads the definition without restating content."""
        steps = "Follow the test-definition for the canonical procedure."
        result = self._run(steps)
        self.assertTrue(result.passed,
            f"clean skill should pass; got: {result.detail}")

    def test_full_block_duplication_fails(self):
        """Skill restates 4+ consecutive lines from the definition verbatim."""
        # Copy 4 meaningful lines from DEFINITION_BODY into the skill body
        steps = textwrap.dedent("""\
            Inline copy of the definition's content (this should fail):

            The skill must apply the canonical safety guard before mutating state.
            Every consumer skill is required to follow the safe-default protocol.
            Failures must surface a warning to the operator and halt the work.
            On retry the skill should re-check whether the precondition still holds.
            """)
        result = self._run(steps)
        self.assertFalse(result.passed,
            "verbatim 4-line block should fail the check")
        self.assertIn("test-definition.md", result.detail)

    def test_short_phrase_echo_passes(self):
        """A 1-2 line phrase echo from the definition does NOT fail the check."""
        steps = textwrap.dedent("""\
            Some skill-specific prose here that talks about verification.

            The skill must apply the canonical safety guard before mutating state.
            But the rest of this section is genuinely skill-specific and
            does not echo any further lines from the test-definition body.
            We pivot to a different topic and stay on it for the duration.
            """)
        result = self._run(steps)
        self.assertTrue(result.passed,
            f"single-line echo should pass; got: {result.detail}")


if __name__ == "__main__":
    unittest.main()
