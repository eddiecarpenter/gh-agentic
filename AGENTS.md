@RULEBOOK.md
@LOCALRULES.md

---

# Session Bootstrap — MANDATORY before your first response

**Your first action in any session is to execute the `session-init` skill at `skills/session-init/SKILL.md` — before producing any user-facing message.** "Session start" means the first agent turn of any conversation, including conversations resumed from a context summary. The user's first message is *always* a session-start signal — whether it's a casual greeting like "hi" or a substantive question. Bootstrap runs either way; only the menu in step 5 differs (shown for casual greetings, skipped when the user has already declared intent).

This rule supersedes conversational instinct. Bootstrap first; then address the user's actual content within or after the bootstrap output.
