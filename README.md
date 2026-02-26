# LLM-Seeker

## Discover OpenAI-compatible LLM endpoints

<img src="images/llm-seeker.png" alt="llm-seeker" width="700">

### Install
```bash
Requires Go.
Install:
go install github.com/fixdot/llm-seeker@latest
```

### Usage

```bash
Ensure your local OpenAI-compatible LLM server is running.
llm-seeker
```

### Example output
```text
http://localhost:11434/v1
http://127.0.0.1:11434/v1
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

