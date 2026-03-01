# LLM-Seeker

## Automatically discover OpenAI-compatible local LLM endpoints

<img src="images/llm-seeker.png" alt="llm-seeker" width="700">

### Overview
llm-seeker is a tiny CLI utility that automatically discovers OpenAI-compatible LLM endpoints on your machine.  

Useful when working with Ollama, LM Studio, vLLM, or any local LLM server.  

It also detects Foundry Local (dynamic port) when the `foundry` CLI is available.

### Why llm-seeker?
llm-seeker scans common local ports and prints a usable base URL — without running a proxy.
By default, only the first detected endpoint is returned.
Use LLM_SEEKER_ORDER to enumerate multiple candidates (one per line).

### Performance note
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

### Install
Requires Go 1.18+

Install:

```bash
go install github.com/fixdot/llm-seeker@latest
```
The binary will be installed:

```text
`$GOPATH/bin` (usually `~/go/bin`)
```

### Default behavior
- Hosts: `127.0.0.1` (listing mode also checks `localhost`)
- Ports: 11434, 1234, 8000, 8080 (override via `LLM_SEEKER_PORTS`)
- Probes: `/v1/models` then `/models`
- Timeout: 800ms per probe (Foundry Local detection may be slower due to CLI overhead)
- Output: first detected base URL

### Usage
Make sure your local OpenAI-compatible LLM server is running.
```bash
llm-seeker
```

### Example (default)
```text
http://127.0.0.1:11434/v1
```

### Example (listing mode)
```text
LLM_SEEKER_ORDER=ollama llm-seeker
```
```text
http://127.0.0.1:11434/v1
http://localhost:11434/v1
```
```text
LLM_SEEKER_ORDER=ollama,lmstudio llm-seeker
```
```text
http://127.0.0.1:11434/v1
http://localhost:11434/v1
http://127.0.0.1:1234/v1
http://localhost:1234/v1
```
### Supported providers

- `ollama` — default port 11434
- `lmstudio` — default port 1234
- `vllm` — default port 8000 (configurable)
- `generic` — default port 8080
- `foundry` — dynamic port (via `foundry service status`)

Ports can be overridden via `LLM_SEEKER_PORTS`.
### Example
```bash
export OPENAI_BASE_URL="$(llm-seeker | head -n 1)"
```

### Exit codes
0 — endpoint found  
1 — no endpoint found  
2 — internal error

### Environment
LLM_SEEKER_PORTS — override the default port list (comma-separated)  
Example: `LLM_SEEKER_PORTS=11434,1234`

LLM_SEEKER_ORDER — control scan order and enumerate specific providers  
Supported values: ollama, lmstudio, vllm, generic, foundry  
Example: `LLM_SEEKER_ORDER=ollama,lmstudio`

### Detection Notes

- Detection relies on HTTP reachability and OpenAI-compatible JSON responses.
- In rare cases (startup timing, port changes, or external CLI behavior), detection may temporarily fail.
- Services using dynamic ports (e.g., Foundry Local) may change endpoints after restart.
- Only the first detected endpoint is returned by default.
  Use LLM_SEEKER_ORDER or LLM_SEEKER_PORTS for enumeration.
- Foundry Local detection is opt-in.
  To avoid slow startup when the service is not running,
  llm-seeker only checks Foundry when explicitly included via `LLM_SEEKER_ORDER`.

Example:

```bash
LLM_SEEKER_ORDER=foundry llm-seeker
```

### License
MIT License

### Links
**Project page**  
https://github.com/fixdot/llm-seeker

