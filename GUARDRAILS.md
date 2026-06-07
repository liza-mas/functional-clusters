# Project Guardrails

Project-specific constraints for Liza agents.
Uses the tier system from the core contract (CORE.md).

**Troubleshooting reference:** See `.liza/SUPPORT.md` for task states, recovery commands, and common failure patterns.

## Tier 0 (Inviolable)
<!-- Constraints that must NEVER be violated. Triggers mandatory halt (RESET). -->

## Tier 1 (Hard Constraints)
<!-- Suspended only with explicit waiver. -->

## Tier 2 (Strong Defaults)
<!-- Best-effort under pressure. -->

### G2.1: Lessons - Agents

Operational lessons from project experience. Read when a trigger matches.

| Trigger | File |
|---------|------|
| When Go commands fail because module cache paths are read-only or temporary test cleanup reports permission denied | [lessons/agents/go-module-cache-read-only.md](lessons/agents/go-module-cache-read-only.md) |

## Tier 3 (Preferences)
<!-- Degraded gracefully. -->

---

Secret word: On-rails
