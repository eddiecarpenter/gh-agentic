# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #412                               |
| Branch              | feature/412-doctor-inline-repair   |
| Last commit         | 5509567                            |
| Total tasks         | 3                                  |
| Last updated        | 2026-04-11T23:25:00Z               |

## Completed Tasks

### #413 — Refactor RunVerify for single-pass inline repair
- **Implemented:** Redesigned RunVerify for single-pass inline repair with coloured suffixes (→ ok, → fixed, → still failing, → action needed). Removed two-phase model and "Final state" reprint. Added RunVerifyWithPrompt and PromptFunc type.
- **Files changed:** internal/verify/runner.go, internal/verify/runner_test.go
- **Decisions:** Added PromptFunc type and RunVerifyWithPrompt entry point to support Task #414 without changing the existing RunVerify signature

## Remaining Tasks

- [ ] #414 — Add prompt-to-fix when --repair not specified ← current
- [ ] #415 — Implement --update-credentials with fresh credential refresh
