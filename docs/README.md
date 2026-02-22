# LLM-Seeker

## Discover OpenAI-compatible LLM endpoints

<img src="images/llm-seeker.png" alt="llm-seeker" width="400">

### Install
```
Requires Go.
1. Install Go  
2. Run:
go install github.com/fixdot/llm-seeker@latest
After installation, ensure your Go bin directory is in PATH.
```

### Usage

```
Ensure your local OpenAI-compatible LLM server is running.
Run in Terminal:
llm-seeker

Example output:
http://localhost:11434/v1
http://127.0.0.1:11434/v1
```

### Output

Prints detected base_url(s) to stdout (newline-separated)

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

**Releases**  
https://github.com/fixdot/llm-seeker/releases

