# ep — API Endpoint Discovery Engine

> **Version:** v1.0  
> **Developer:** INTELEON  
> **Language:** Go

A fast, concurrent, and security-hardened API endpoint discovery scanner.

---

## Features

- **Multi-Method Support** — GET, POST, PUT, DELETE, HEAD, OPTIONS, PATCH
- **Request Body Support** — Send JSON/XML payloads with POST/PUT/PATCH
- **Rate-Limit Resilience** — Automatic retry with exponential backoff on `429`
- **Secure by Default** — TLS verification enabled; `--insecure` is opt-in
- **Memory Safe** — Response bodies capped at 10MB (configurable)
- **Redirect Control** — Choose whether to follow HTTP redirects
- **Clean Shutdown** — Graceful exit on `Ctrl+C` / SIGTERM
- **OPSEC Friendly** — Generic User-Agent by default, fully customizable

---

## Installation

```bash
git clone https://github.com/inteleon404/ep.git
cd ep
go build -o ep main.go
```

Or install directly:

```bash
go install github.com/inteleon/ep@latest
```

---

## Usage

```bash
ep -u <URL> -w <WORDLIST> [FLAGS]
```

### Basic Scan
```bash
ep -u https://api.example.com -w endpoints.txt
```

### POST with JSON Body
```bash
ep -u https://api.example.com -w endpoints.txt \
   -m POST \
   -d '{"key":"value"}' \
   -H "Content-Type: application/json"
```

### With Proxy & Custom Threads
```bash
ep -u https://api.example.com -w endpoints.txt \
   -p http://127.0.0.1:8080 \
   -t 50 \
   --delay 100
```

### Silent Mode (URLs only)
```bash
ep -u https://api.example.com -w endpoints.txt -silent
```

---

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-u, --url` | Target base URL | *(required)* |
| `-w, --wordlist` | Path to wordlist file | *(required)* |
| `-m, --method` | HTTP method | `GET` |
| `-d, --data` | Request body (for POST/PUT/PATCH) | `""` |
| `-H, --header` | Custom HTTP header (repeatable) | — |
| `-p, --proxy` | Proxy URL (`http://` or `socks5://`) | — |
| `-t, --threads` | Concurrent worker threads | `20` |
| `--delay` | Delay between requests (ms) | `0` |
| `--timeout` | Request timeout (seconds) | `10` |
| `--max-size` | Max response body size (bytes) | `10485760` (10MB) |
| `--ignore-code` | Comma-separated status codes to ignore | `404` |
| `--insecure` | Skip TLS certificate verification | `false` |
| `--follow-redirects` | Follow HTTP 3xx redirects | `false` |
| `--user-agent` | Custom User-Agent string | Generic Mozilla |
| `-silent` | Output only discovered endpoints | `false` |
| `-v, --verbose` | Show detailed error logs | `false` |
| `-h, --help` | Show help and exit | — |

---

## Wordlist Format

Plain text file, one endpoint per line. Leading `/` is optional. Empty lines and `#` comments are ignored.

```
# API endpoints
admin
api/v1/users
api/v1/login
internal/health
swagger-ui.html
```

---

## Security Notes

- **TLS verification is ON by default.** Use `--insecure` only in controlled lab environments.
- **Response bodies are capped** to prevent memory exhaustion from malicious servers.
- **Rate-limiting (`429`) is handled gracefully** with automatic retries and backoff.

---

## License

MIT License © INTELEON
