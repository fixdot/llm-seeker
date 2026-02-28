# LLM-Seeker

## Automatically discover OpenAI-compatible local LLM endpoints

<img src="images/llm-seeker.png" alt="llm-seeker" width="700">

### Overview

llm-seeker is a tiny CLI utility that automatically discovers OpenAI-compatible LLM endpoints on your machine.  

Useful when working with Ollama, LM Studio, vLLM, or any local LLM server.  

It also detects Foundry Local (dynamic port) when the `foundry` CLI is available.

### Why llm-seeker?

When running multiple local LLM servers (Ollama, LM Studio, vLLM, Foundry Local),
remembering which port currently exposes an OpenAI-compatible endpoint is annoying.

llm-seeker scans common localhost ports and prints a usable base URL — without running a proxy.

### Install
Requires Go 1.18+  
Install:

```bash
go install github.com/fixdot/llm-seeker@latest
```
The binary will be installed to $GOPATH/bin (usually ~/go/bin).
### Default behavior

- Hosts: localhost, 127.0.0.1
- Ports: 11434, 1234, 8000, 8080 (override via `LLM_SEEKER_PORTS`)
- Probes: /v1/models then /models
- Timeout: 800ms per probe
- Output: base URL(s), one per line

### Usage

Make sure your local OpenAI-compatible LLM server is running.
```bash
llm-seeker
```

### Example output
```text
http://localhost:11434/v1
http://127.0.0.1:11434/v1
```

### Example

```bash
export OPENAI_BASE_URL="$(llm-seeker | head -n 1)"
```

### Exit codes

0 — endpoint found  
1 — no endpoint found  
2 — internal error

### Environment

LLM_SEEKER_PORTS — override default port list (comma-separated)

### License

MIT License

### Links

**Project page**  
https://github.com/fixdot/llm-seeker

