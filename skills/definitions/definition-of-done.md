# Definition of Done

Fixes the meaning of the term **done** as it applies to any skill in
this framework.

## The term

A skill is **done** when, at the moment control would return to its
caller, all three of the following hold:

1. **Artefact existence.** Every item in the skill's `## Output
   Artefacts` section exists and matches the observable property
   declared alongside it (the file path, the comment header pattern,
   the label state, the branch name, the structured return shape —
   whatever the artefact entry named).
2. **Verification pass.** Every check in the skill's `## Verification`
   section returns a passing result. Both layers count: mechanical
   and semantic.
3. **No unhandled error of severity ≥ ERROR.** Any error of `ERROR`
   or `FATAL` severity raised during execution must have been handled
   by the skill, or it is not done — it has halted.

If any of the three fails, the skill is not done. The correct
response is to halt per the framework's error model, not to declare
done.

## Why all three

- Artefacts alone aren't enough: a file can exist with the wrong
  content. The Verification section is what catches that.
- Verification alone isn't enough: a skill can pass its checks while
  having silently skipped producing an artefact it declared. Output
  Artefacts is the source-of-truth list; Verification is the proof.
- Error-clean alone isn't enough: a skill can finish without errors
  while having produced none of its declared work (it took an
  early-exit path that wasn't actually a success).

The three together are the contract. Any one of them missing leaves
a hole the agent can slip through under delivery pressure.

## "Done" is a runtime self-test, not a declaration

Done is established by the skill *running its own Verification
section* and observing the result. A skill that says "I am done"
without running its checks has not established done — it has
asserted it.

This is the property that makes "done" mechanically meaningful: the
same checks that prove done in production prove "the test scenario
succeeded" in evaluation. The check is the contract, not the claim.

## Relationship to other terms

- **Halted** — the skill stopped before reaching done. Halting is a
  legitimate outcome (errors, user cancellation, alternative path);
  it is the *opposite* of done, not a kind of done.
- **Complete** — informal synonym for done. This document fixes
  "done" as the canonical term; "complete" is allowed in prose but
  carries no additional precision.
- **Successful** — outcome-oriented framing of the same state. A
  skill that is done is successful; a skill that is halted may or
  may not be (a planned alternative path is a successful halt).
