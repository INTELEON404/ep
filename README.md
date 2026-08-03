# ep

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License">
  <img src="https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-8A2BE2" alt="Platform">
</p>

<p align="center">
  <b>Fast, concurrent API endpoint discovery engine</b><br>
  <sub>Built for speed. Designed for recon.</sub>
</p>

---

## Overview

`ep` is a lightweight, high-performance endpoint discovery scanner written in Go. It brute-forces API endpoints using a wordlist with configurable concurrency, intelligent retry logic, and clean output formatting suitable for both interactive use and pipeline integration.

---

## Features

| Feature | Description |
|---------|-------------|
| **Multi-Target** | Scan single URL (`-u`) or a list of URLs (`-i`) |
| **Concurrent Scanning** | Configurable thread pool for high-speed enumeration |
| **Multi-Method Support** | GET, POST, PUT, DELETE, HEAD, OPTIONS, PATCH |
| **Request Body Injection** | Send custom payloads with POST/PUT/PATCH |
| **Smart Retry Logic** | Exponential backoff on `429 Too Many Requests` |
| **Response Capping** | Memory-safe body reads (default 10MB, configurable) |
| **Redirect Control** | Follow or ignore HTTP 3xx redirects |
| **Proxy Support** | HTTP/HTTPS/SOCKS5 proxy compatibility |
| **Match & Filter** | Match specific status codes (`-mc`) or ignore others |
| **Field Selection** | Toggle status-code, content-length, response-time, server header |
| **Silent Mode** | Clean URL-only output for piping into other tools |
| **File Output** | Save results to file (`-o`) while still printing to stdout |
| **Graceful Shutdown** | Handles `Ctrl+C` / SIGTERM without corrupting output |
| **OPSEC Friendly** | Custom User-Agent and header support |

---

## Installation

### From Source

```bash
git clone https://github.com/inteleon404/ep.git
cd ep
go build -o ep ep.go
sudo mv ep /usr/local/bin/
```

### Using `go install`

```bash
go install github.com/inteleon404/ep@latest
```

### Precompiled Binaries

Download the latest release from the [Releases](https://github.com/inteleon404/ep/releases) page.

---

## Usage

```bash
ep -u <TARGET_URL> -w <WORDLIST> [FLAGS]
ep -i <URL_LIST>  -w <WORDLIST> [FLAGS]
```

### Quick Start

```bash
# Basic single-target scan
ep -u https://api.target.com -w endpoints.txt

# Multi-target scan from file
ep -i urls.txt -w endpoints.txt

# Match only 200 and 301 responses
ep -u https://api.target.com -w endpoints.txt -mc 200,301

# POST scan with JSON payload
ep -u https://api.target.com -w wordlist.txt \
   -m POST \
   -d '{"user":"admin","pass":"test"}' \
   -H "Content-Type: application/json"

# Through proxy with custom threads and delay
ep -u https://api.target.com -w wordlist.txt \
   -p http://127.0.0.1:8080 \
   -t 50 \
   --delay 100 \
   --insecure

# Silent mode — pipe into other tools
ep -u https://api.target.com -w wordlist.txt -silent | httpx -sc

# Show response time and server header
ep -u https://api.target.com -w wordlist.txt -rt -server

# Save results to file
ep -u https://api.target.com -w wordlist.txt -o results.txt
```

---

## Flags

### Input

| Flag | Description | Default |
|------|-------------|---------|
| `-u, --url` | Target base URL | *(optional if `-i` used)* |
| `-i, --input` | File containing target URLs (one per line) | *(optional if `-u` used)* |
| `-w, --wordlist` | Path to wordlist file | *(required)* |

### Request

| Flag | Description | Default |
|------|-------------|---------|
| `-m, --method` | HTTP method | `GET` |
| `-d, --data` | Request body (for POST/PUT/PATCH) | `""` |
| `-H, --header` | Custom HTTP header (repeatable) | — |
| `-p, --proxy` | Proxy URL (`http://` / `socks5://`) | — |
| `--user-agent` | Custom User-Agent string | Generic Mozilla |
| `--follow-redirects` | Follow HTTP 3xx redirects | `false` |
| `--insecure` | Skip TLS certificate verification | `false` |

### Performance

| Flag | Description | Default |
|------|-------------|---------|
| `-t, --threads` | Concurrent worker threads | `20` |
| `--delay` | Delay between requests (milliseconds) | `0` |
| `--timeout` | Request timeout (seconds) | `10` |
| `--retries` | Max retries on failure | `3` |
| `--max-size` | Max response body size (bytes) | `10485760` (10MB) |

### Filtering

| Flag | Description | Default |
|------|-------------|---------|
| `--ignore-code` | Comma-separated status codes to ignore | `404` |
| `-mc, --match-code` | Comma-separated status codes to match (e.g. `200,301,302`) | — |

### Output

| Flag | Description | Default |
|------|-------------|---------|
| `-sc, --status-code` | Show status code in output | `true` |
| `-cl, --content-length` | Show content length in output | `true` |
| `-rt, --response-time` | Show response time | `false` |
| `-server` | Show `Server` header | `false` |
| `-silent` | Output only discovered endpoints (no banner, no colors) | `false` |
| `-o, --output` | Save results to file | — |
| `-v, --verbose` | Show detailed error/retry logs | `false` |
| `-h, --help` | Display help and exit | — |

---

## Wordlist Format

Plain text file. One endpoint per line. Leading `/` is optional. Empty lines and `#` comments are ignored.

```text
# API endpoints
admin
api/v1/users
api/v1/auth/login
internal/health
swagger-ui.html
actuator/env
```

---

## Output Format

**Standard Mode:**
```
[200] [GET] https://api.target.com/api/v1/users [Size: 1240]
[301] [GET] https://api.target.com/admin [Size: 0]
[500] [GET] https://api.target.com/internal/health [Size: 512]
```

**With `-rt -server`:**
```
[200] [GET] https://api.target.com/api/v1/users [Size: 1240] [Time: 45ms] [Server: nginx]
```

**Silent Mode:**
```
https://api.target.com/api/v1/users
https://api.target.com/admin
```

---

## Security & Safety

- **TLS verification is enabled by default.** Use `--insecure` only in controlled lab environments.
- **Response bodies are hard-capped** to prevent memory exhaustion from malicious or misconfigured servers.
- **Rate-limiting (`429`) is handled gracefully** with automatic retries and exponential backoff.
- **No data exfiltration.** The tool makes no external connections beyond your specified target and proxy.

---

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## License

MIT License © 2026 INTELEON

See [LICENSE](LICENSE) for full details.

---

<p align="center">
  <sub>Built with precision. Used with intent.</sub>
</p>
