# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #412                               |
| Branch              | feature/412-doctor-inline-repair   |
| Last commit         | 4acdc9c                            |
| Total tasks         | 3                                  |
| Last updated        | 2026-04-11T23:32:00Z               |

## Completed Tasks

### #413 — Refactor RunVerify for single-pass inline repair
- **Implemented:** Redesigned RunVerify for single-pass inline repair with coloured suffixes (→ ok, → fixed, → still failing, → action needed). Removed two-phase model and "Final state" reprint.
- **Files changed:** internal/verify/runner.go, internal/verify/runner_test.go
- **Decisions:** Added PromptFunc type and RunVerifyWithPrompt entry point

### #414 — Add prompt-to-fix when --repair not specified
- **Implemented:** Added interactive prompt-to-fix flow with VerifyOptions struct. When no --repair and issues found, prompts user. Selective repair shows only issue lines with suffixes. --yes auto-confirms.
- **Files changed:** internal/verify/runner.go, internal/verify/runner_test.go, internal/cli/doctor.go
- **Decisions:** Introduced VerifyOptions struct to cleanly separate inline-repair and prompt-to-fix modes

## Remaining Tasks

- [ ] #415 — Implement --update-credentials with fresh credential refresh ← current
