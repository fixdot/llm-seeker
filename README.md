# LLM-Seeker

## Automatically discover OpenAI-compatible LLM endpoints on your machines

<img src="images/llm-seeker.png" alt="llm-seeker" width="700">

---

## Quick Start

Install:

```bash
go install github.com/fixdot/llm-seeker@latest
```

Run:

```bash
llm-seeker
```

Example output:

```text
http://127.0.0.1:11434/v1
```

Set environment variable automatically:

```bash
eval "$(llm-seeker --env)"
```

---

## Overview

llm-seeker is a tiny CLI utility that automatically discovers OpenAI-compatible LLM endpoints on your machine.

Useful when working with Ollama, LM Studio, vLLM, or any local LLM server.

It also detects Foundry Local (dynamic port) when explicitly requested.

---

## Why llm-seeker?

llm-seeker scans common local ports and prints a usable base URL — without running a proxy.

By default, only the first detected endpoint is returned (optimized for shell integration).

Use LLM_SEEKER_ORDER to list multiple candidates with provider labels.

---

## Performance Note

llm-seeker scans a set of common local ports by default.

If detection feels slow (e.g. due to inactive ports or Foundry CLI overhead),  
you can limit the scan using:

```bash
LLM_SEEKER_PORTS=11434,1234 llm-seeker
```

You can also control provider scanning using:

```bash
LLM_SEEKER_ORDER=ollama,lmstudio llm-seeker
```

---

## Install

Requires Go 1.18+

```bash
go install github.com/fixdot/llm-seeker@latest
```

The binary will be installed in:

```bash
$GOPATH/bin (usually ~/go/bin)
```

---

## Default Behavior

- Hosts: 127.0.0.1 (listing mode also checks localhost)
- Ports: 11434, 1234, 8000, 8080 (override via LLM_SEEKER_PORTS)
- Probes: /v1/models
- HTTP timeout: 800ms per probe
- Output: first detected base URL
- Foundry is not scanned by default

---

## Usage

Make sure your local OpenAI-compatible LLM server is running.

```bash
llm-seeker
```

You can also output a shell export command:

```bash
llm-seeker --env
```

---

## Example (default)

```text
http://127.0.0.1:11434/v1
```

---

## Example (listing mode)

```text
LLM_SEEKER_ORDER=ollama llm-seeker

ollama http://127.0.0.1:11434/v1
ollama http://localhost:11434/v1

LLM_SEEKER_ORDER=ollama,lmstudio llm-seeker

ollama http://127.0.0.1:11434/v1
ollama http://localhost:11434/v1
lmstudio http://127.0.0.1:1234/v1
lmstudio http://localhost:1234/v1
```

---

## Example (--env)

```bash
llm-seeker --env

export OPENAI_BASE_URL=http://127.0.0.1:11434/v1
```

---

## Supported Servers

| Provider | Default Port | Detection Method |
|----------|-------------|------------------|
| Ollama | 11434 | HTTP probe |
| LM Studio | 1234 | HTTP probe |
| vLLM | 8000 | HTTP probe |
| Generic OpenAI API | 8080 | HTTP probe |
| Foundry Local | dynamic | foundry service status |

Ports can be overridden via LLM_SEEKER_PORTS.

---

## Shell Integration

Recommended:

```bash
eval "$(llm-seeker --env)"
```

Alternative:

```bash
export OPENAI_BASE_URL="$(llm-seeker | head -n 1)"
```

---

## Exit Codes

```text
0 — endpoint found
1 — no endpoint found
2 — invalid input
```

---

## Environment

### LLM_SEEKER_PORTS

Override the default port list (comma-separated)

Example:

```bash
LLM_SEEKER_PORTS=11434,1234
```

### LLM_SEEKER_ORDER

Set scan priority and/or limit providers.

- Acts as both priority and filter
- Enables listing mode (provider + URL)
- Takes precedence over LLM_SEEKER_PORTS

Supported values:

```text
ollama
lmstudio
vllm
generic
foundry
```

Example:

```bash
LLM_SEEKER_ORDER=ollama,lmstudio
```

---

## Detection Notes

- Detection relies on HTTP reachability and OpenAI-compatible JSON responses.
- In rare cases (startup timing, port changes, or external CLI behavior), detection may temporarily fail.
- Services using dynamic ports (e.g., Foundry Local) may change endpoints after restart.
- Only the first detected endpoint is returned by default.
- Use LLM_SEEKER_ORDER or LLM_SEEKER_PORTS for enumeration.
- Foundry Local detection is opt-in. To preserve fast default execution, llm-seeker checks Foundry only when explicitly included via LLM_SEEKER_ORDER.
- When Foundry is included but not running, llm-seeker fails fast (~4 seconds).
- Foundry endpoint detection uses a file-based cache (TTL 60 minutes), automatically validated via HTTP probe.

Example:

```bash
LLM_SEEKER_ORDER=foundry llm-seeker
```

---

## License

MIT License

---

## Links

Project page  
https://github.com/fixdot/llm-seeker