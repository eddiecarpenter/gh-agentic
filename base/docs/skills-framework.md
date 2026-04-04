# Skills Framework

## What is a Skill?

A skill is a well-formed, reusable prompt that instructs an AI agent to achieve
a specific goal. Skills are LLM-agnostic — written to work with any capable
model without vendor-specific syntax or assumptions.

Skills are the **portable intellectual property** of the agentic development
framework. They encode the process knowledge for each phase of software delivery.

## Architecture

```
Skill (base/skills/*.md)
  └── wrapped by Goose Recipe (.goose/recipes/*.yaml)
        └── triggered by GitHub Actions (.github/workflows/*.yml)
```

- The **skill** defines what the agent does — the intelligence
- The **recipe** defines how to run it — provider, model, tools, parameters
- The **workflow** defines when to run it — triggers, branch management, PR lifecycle

Each layer is independently swappable. Replacing Goose with another frontend
requires only rewrapping the skills. The skills themselves remain unchanged.

## Authoring Rules

- Write skills as clear briefs for any intelligent collaborator
- No vendor-specific syntax — no Claude-specific formatting, no Goose-specific references
- No model assumptions — do not assume a specific context window, reasoning style, or API
- One skill per phase — single responsibility
- Skills reference `AGENTS.md` as the source of truth for process rules

## Governance

**Skills are read-only.** Never modify a skill locally.

- Customisation of agent behaviour belongs in `AGENTS.local.md`
- If a skill needs to change, raise it against `eddiecarpenter/agentic-development`
  and let it flow in via `gh agentic sync`
- `gh agentic verify` detects and flags any local modifications to `base/skills/*.md`

## Relationship to Recipes

The Goose recipe (`.goose/recipes/*.yaml`) wraps a skill with execution context:

```yaml
# The recipe provides runtime configuration
version: "1.0.0"
title: "Dev Session (Stage 4)"
extensions:
  - type: builtin
    name: developer
settings:
  max_turns: 200
parameters:
  - key: feature_issue

# The instructions field IS the skill
instructions: |
  You are running a Dev Session...
```

The `instructions` field contains the skill. Everything else is recipe infrastructure.
In the current implementation the skill is embedded directly in the recipe YAML.
The `base/skills/*.md` files are the human-readable reference documentation for each skill.
