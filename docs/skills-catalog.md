# Skills Catalog

OpenTide ships with 10 built-in skills. Each runs in an isolated container with declared security constraints.

## No-Egress Skills (fully isolated)

These skills have no network access. They cannot reach any external service.

### web-search
Search the web using the Brave Search API.

| Property | Value |
|----------|-------|
| Tool name | `web_search` |
| Egress | `api.brave.com:443` |
| Memory | 128Mi |
| Timeout | 30s |
| Config | `BRAVE_API_KEY` (required) |

### calculator
Evaluate mathematical expressions safely.

| Property | Value |
|----------|-------|
| Tool name | `calculator` |
| Egress | None |
| Memory | 64Mi |
| Timeout | 5s |

### file-manager
Read, write, and list files in the skill's sandbox.

| Property | Value |
|----------|-------|
| Tool name | `file_manager` |
| Egress | None |
| Filesystem | read-write (tmpfs only) |
| Memory | 128Mi |
| Timeout | 10s |

### text-tools
Text manipulation: word count, character count, case conversion, reverse, trim, replace, truncate.

| Property | Value |
|----------|-------|
| Tool name | `text_tools` |
| Egress | None |
| Memory | 64Mi |
| Timeout | 5s |

**Operations:** `word_count`, `char_count`, `uppercase`, `lowercase`, `title_case`, `reverse`, `trim`, `replace`, `truncate`

### json-tools
JSON utilities: validate, format, minify, extract keys, check types, query with dot notation.

| Property | Value |
|----------|-------|
| Tool name | `json_tools` |
| Egress | None |
| Memory | 64Mi |
| Timeout | 5s |

**Operations:** `validate`, `format`, `minify`, `keys`, `type`, `query`

### reminder
Set time-based reminders delivered via the messaging adapter.

| Property | Value |
|----------|-------|
| Tool name | `reminder` |
| Egress | None |
| Memory | 64Mi |
| Timeout | 5s |

**Format:** "remind me in 30m to check the build" or "reminder: 2h deploy to production"

## Egress Skills (declared network access)

These skills can reach specific external APIs. All other egress is blocked.

### url-fetch
Fetch and return content from any HTTPS URL.

| Property | Value |
|----------|-------|
| Tool name | `url_fetch` |
| Egress | `*:443` (any HTTPS) |
| Memory | 256Mi |
| Timeout | 30s |

**Security:** Full SSRF protection. Blocks private IPs (RFC1918, RFC4291, link-local, cloud metadata at 169.254.169.254). DNS resolution checked before connect and at each redirect hop. Max 5 redirects. Response capped at 1MB.

### github
Read-only GitHub API access: issues, pull requests, commits, repo info.

| Property | Value |
|----------|-------|
| Tool name | `github_api` |
| Egress | `api.github.com:443` |
| Memory | 128Mi |
| Timeout | 30s |
| Config | `GITHUB_TOKEN` (optional, for private repos and higher rate limits) |

**Commands:** `issues owner/repo`, `pulls owner/repo`, `commits owner/repo`, `repo owner/repo`

### image-gen
Generate images using OpenAI's DALL-E 3 API.

| Property | Value |
|----------|-------|
| Tool name | `image_gen` |
| Egress | `api.openai.com:443` |
| Memory | 256Mi |
| Timeout | 60s |
| Config | `OPENAI_API_KEY` (required) |

Returns the generated image URL. Size options: 1024x1024 (default), 1792x1024, 1024x1792.

## Platform-Dependent Skills

### code-runner
Execute Python, JavaScript, or Go code snippets in a sandboxed environment.

| Property | Value |
|----------|-------|
| Tool name | `code_runner` |
| Egress | None (`--network=none`) |
| Memory | 256Mi |
| Timeout | 10s |
| Platform | Requires Docker socket access |

**Not available on DO App Platform.** Works on Droplets, self-hosted, or any environment with Docker daemon access. On platforms without Docker, this skill is automatically skipped with a log message.

**Languages:** Python 3, JavaScript (Node.js), Go. Auto-detects language from code content or explicit prefix (`python:`, `js:`, `go:`).

**Security:** Each execution runs in a fresh container with `--network=none`, tmpfs-only filesystem, 256Mi memory limit, and 10s hard kill timeout. No access to host environment variables or filesystem. Output capped at 64KB.

## Creating Custom Skills

Scaffold a new skill:

```bash
tide-cli skill new my-skill
```

This creates `skills/my-skill/` with:
- `skill.yaml` - manifest (security constraints, triggers)
- `main.go` - skill logic (reads JSON from stdin, writes JSON to stdout)
- `go.mod` - Go module
- `Dockerfile` - container build

### Skill Contract

Skills communicate via stdin/stdout JSON:

**Input:**
```json
{"arguments": {"query": "user's request"}}
```

**Output (success):**
```json
{"content": "result text"}
```

**Output (error):**
```json
{"error": "error message"}
```

### Security Manifest

Every skill declares its security posture in `skill.yaml`:

```yaml
security:
  egress:
    - "api.example.com:443"    # Only these hosts reachable
  filesystem: read-only         # No persistent writes
  max_memory: 128Mi             # Memory limit
  max_cpu: "0.5"                # CPU limit
  timeout: 30s                  # Hard kill timeout
```

Skills with `egress: []` (empty) have no network access at all.
