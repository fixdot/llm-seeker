# LLM-Seeker

## Automatically discover OpenAI-compatible LLM endpoints on your machines


<img src="images/llm-seeker.png" alt="llm-seeker" width="992">

## Overview

llm-seeker is a tiny CLI utility that automatically discovers OpenAI-compatible LLM endpoints on your machine. Useful when working with Ollama, LM Studio, vLLM, or other local LLM servers. It can also detect Foundry Local when explicitly requested. It scans common local ports and returns a usable base URL without running a proxy.

## Installation

### Homebrew (macOS, Apple Silicon)

```bash
brew tap fixdot/tap
brew trust --formula fixdot/tap/llm-seeker
brew install fixdot/tap/llm-seeker
```

Update:

```bash
brew upgrade fixdot/tap/llm-seeker
```

Uninstall:

```bash
brew uninstall fixdot/tap/llm-seeker
```

### Go

Requires Go 1.18+.

```bash
go install github.com/fixdot/llm-seeker@latest
```

The executable is installed in `$GOPATH/bin` (usually `~/go/bin`).

## Usage

Make sure your local OpenAI-compatible LLM server is running.

Find the first available endpoint:

```bash
llm-seeker
```

Example output:

```text
http://127.0.0.1:11434/v1
```

Set `OPENAI_BASE_URL` automatically:

```bash
eval "$(llm-seeker --env)"
```

List endpoints from specific providers:

```bash
LLM_SEEKER_ORDER=ollama,lmstudio llm-seeker
```

Example output:

```text
ollama http://127.0.0.1:11434/v1
ollama http://localhost:11434/v1
lmstudio http://127.0.0.1:1234/v1
lmstudio http://localhost:1234/v1
```

## Configuration

### LLM_SEEKER_PORTS

Override the default ports (11434, 1234, 8000, 8080):

```bash
LLM_SEEKER_PORTS=11434,1234 llm-seeker
```

### LLM_SEEKER_ORDER

Set provider priority, limit scanning, and enable listing mode.

Supported values: `ollama`, `lmstudio`, `vllm`, `generic`, `foundry`.

```bash
LLM_SEEKER_ORDER=ollama,lmstudio llm-seeker
```

When set, `LLM_SEEKER_ORDER` takes precedence over `LLM_SEEKER_PORTS`.

Foundry Local detection is opt-in:

```bash
LLM_SEEKER_ORDER=foundry llm-seeker
```

## Supported Servers

| Provider | Default Port | Detection |
|---|---|---|
| Ollama | 11434 | HTTP probe |
| LM Studio | 1234 | HTTP probe |
| vLLM | 8000 | HTTP probe |
| Generic OpenAI API | 8080 | HTTP probe |
| Foundry Local | Dynamic | `foundry service status` |

## Technical Notes

- Scans `127.0.0.1` by default; listing mode also checks `localhost`.
- Probes `/v1/models` with an 800 ms HTTP timeout per probe.
- Returns the first detected endpoint unless listing mode is enabled.
- Detection may temporarily fail during server startup or port changes.
- Foundry Local is excluded from default scans to avoid additional overhead.
- Foundry detection uses a file-based cache (60-minute TTL), validated via HTTP.
- When Foundry is requested but not running, detection fails fast (approximately 4 seconds).

### Exit Codes

| Code | Meaning |
|---|---|
| 0 | Endpoint found |
| 1 | No endpoint found |
| 2 | Invalid input |

## License

LLM-Seeker is released under the MIT License.

Copyright © 2026 fix

## Project

https://github.com/fixdot/llm-seeker
