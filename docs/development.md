# LLM-Seeker beta2 Development Notes (Foundry Local detection issue)



## This document records internal development issues and troubleshooting notes.



This document is not intended for end users.

### Problem

After adding Foundry Local (dynamic port) detection in 0.1.0-beta2:

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

   → Increased timeoutProbe to 3 seconds.

2. Foundry probe logic too strict

   Relying on `/v1/models` probe could fail during warm-up.
   Changed behavior to:

   - If `foundry service status` reports running,
     return `baseURL + "/v1"` immediately.

3. Go build cache confusion

   `go run .` reflected updated code,
   but installed binary still used cached build.

   Fix:
   go clean -cache
   go install .

### Final Stable Configuration

- timeoutProbe = 3 * time.Second
- Foundry detection returns base URL without additional probe
- Clean reinstall performed

Result:
Stable endpoint detection across multiple terminal restarts and repeated executions.

