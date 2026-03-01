# LLM-Seeker Development Notes

This document records internal development issues, design decisions,
and troubleshooting notes.

This document is not intended for end users.

---

## 0.1.0-beta2 — 2026-02-28

### Problem

After adding Foundry Local (dynamic port) detection:

- `go run .` correctly detected Foundry.
- Installed `llm-seeker` sometimes returned:
  llm-seeker: no endpoint found

Even when:

- `foundry service status` showed running.
- `/v1/models` responded correctly.
- PATH included `/opt/homebrew/bin`.

### Root Causes

1. Timeout too short (800ms)

   Foundry `/v1/models` initial response time was ~0.8s.
   800ms caused intermittent false negatives.

   → Increased HTTP probe timeout to 3 seconds.

2. Probe timing instability

   Relying strictly on `/v1/models` could fail during service warm-up.

3. Go build cache confusion

   `go run .` reflected updated code,
   but installed binary still used cached build.

   Fix:
   go clean -cache
   go install .

### Final Stable Configuration (beta2)

- timeoutProbe increased
- Foundry detection stabilized
- Clean reinstall required to avoid stale binaries

---

## 0.1.0-beta3 — 2026-03-01

### Background

Foundry detection in beta2 still caused long delays
when the service was stopped or not fully initialized.

`foundry service status` can block for several seconds,
and retry logic amplified the delay.

In worst cases, default execution became slow
when Foundry was not running.

This contradicted llm-seeker’s design goal:
fast default execution.

### Design Revision

1. Foundry detection is now opt-in via `LLM_SEEKER_ORDER`.

   Default execution no longer probes Foundry.

2. Introduced file-based endpoint cache (TTL 60s).

3. Added CLI timeout (10s) to prevent long blocking.

4. Cache validation uses HTTP probe to ensure correctness.

5. Removed nested retry amplification.

### Final Stable Configuration (beta3)

- Default mode remains extremely fast (~0.02s).
- No long blocking when Foundry is stopped.
- Stable Foundry detection when explicitly requested.
- Automatic recovery when Foundry restarts with a new port.
- Improved overall architectural clarity.
