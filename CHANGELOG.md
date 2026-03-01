# Changelog

All notable changes to this project will be documented in this file.


---

## 0.1.0-beta3 — 2026-03-01

### Changed
- Made Foundry Local detection opt-in via `LLM_SEEKER_ORDER`
  to preserve fast default execution.
- Default execution no longer probes Foundry.

### Added
- File-based cache for Foundry Local endpoint (TTL 60s).
- CLI timeout (10s) for `foundry service status`.
- Automatic cache validation via HTTP probe.

### Fixed
- Eliminated long delays when Foundry was not running.
- Improved stability of Foundry detection across restarts.

---

## 0.1.0-beta2 — 2026-02-28

### Added
- Detect Foundry Local (dynamic port) via `foundry service status`
- `LLM_SEEKER_PORTS` environment variable to override default port list

### Changed
- Updated version to 0.1.0-beta2
- Refined README with clearer Overview, Why, and usage examples

---

## 0.1.0-beta1 — 2026-02-26

### Added
- Initial public release
- Basic OpenAI-compatible endpoint discovery
- Support for Ollama, LM Studio, and vLLM