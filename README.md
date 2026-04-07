# OpenTide

Security-first AI agent platform. Built from scratch in Go.

OpenClaw has 156 security advisories. NemoClaw bolts security onto a broken foundation. OpenTide builds security into the architecture.

## What Makes OpenTide Different

| | OpenClaw | NemoClaw | OpenTide |
|---|---|---|---|
| Security model | Opt-in, bypassable | Bolt-on, alpha | Built-in, mandatory |
| Skill isolation | Host process | Optional containers | Mandatory sandbox |
| Network egress | Allow all | Optional allowlist | Block all by default |
| Approval model | Bypassable (9 CVEs) | Wrapper-level | Hash-verified, TOCTOU-safe |
| LLM Providers | 2 | NVIDIA NIMs | Anthropic + OpenAI + DO Gradient |
| Language | Node.js | Node.js | Go |

## Quick Start (2 minutes)

```bash
git clone https://github.com/opentide/opentide.git && cd opentide
export OPENTIDE_ADMIN_SECRET=$(go run ./cmd/tide-cli admin secret 2>/dev/null)
export ANTHROPIC_API_KEY=sk-ant-your-key  # or add via admin UI after startup
go run ./cmd/opentide --demo
```

## Discord Bot

1. Create app at https://discord.com/developers/applications
2. Bot tab: copy token, enable **Message Content Intent**
3. OAuth2 > URL Generator: scopes `bot` + `applications.commands`, permissions: Send Messages, Read History, Slash Commands
4. Open the generated URL to invite the bot to your server

```bash
export OPENTIDE_ADMIN_SECRET=$(go run ./cmd/tide-cli admin secret 2>/dev/null)
export DISCORD_TOKEN=your-bot-token
export ANTHROPIC_API_KEY=sk-ant-your-key  # or add via admin UI
go run ./cmd/opentide
```

@mention the bot in channels, use `/chat`, or DM it directly.

## Multi-Provider Routing

Use multiple LLM providers simultaneously. Route by channel, switch mid-conversation:

```bash
export ANTHROPIC_API_KEY=sk-ant-your-key
export OPENAI_API_KEY=sk-your-key
go run ./cmd/opentide --demo
```

Users switch providers in chat with `/model anthropic`, `/model openai`, `/model reset`.

Configure channel-based routing in `opentide.yaml` so #engineering uses Claude while #general uses GPT-4o. See [Provider Routing](docs/provider-routing.md).

## 10 Built-in Skills

Every skill runs in an isolated container with declared security constraints. No host process execution.

| Skill | What it does | Network access |
|-------|-------------|----------------|
| web-search | Brave Search API | `api.brave.com:443` |
| calculator | Math expressions | None |
| file-manager | Sandboxed file I/O | None |
| text-tools | Case, count, trim, replace | None |
| json-tools | Validate, format, query | None |
| reminder | Time-based reminders | None |
| url-fetch | Fetch URLs (SSRF-protected) | `*:443` |
| github | Issues, PRs, commits (read-only) | `api.github.com:443` |
| image-gen | DALL-E 3 image generation | `api.openai.com:443` |
| code-runner | Python/JS/Go sandbox | None (`--network=none`) |

Scaffold a new skill: `tide-cli skill new my-skill`

See the full [Skills Catalog](docs/skills-catalog.md).

## Admin Dashboard

Security control plane at `http://localhost:8080/admin/`:

- Live security event feed (approvals, denials, TOCTOU mismatches)
- Audit log with side-by-side payload diffs
- Egress posture visualization per skill
- Provider routing management
- Approval policy configuration

```bash
export OPENTIDE_ADMIN_SECRET=$(tide-cli admin secret)
go run ./cmd/opentide
# Open http://localhost:8080/admin/
```

## Deploy to DigitalOcean

```bash
doctl apps create --spec deploy/app-platform/app.yaml
```

Set env vars in the DO dashboard: `OPENTIDE_ADMIN_SECRET` and `DISCORD_TOKEN` (or `SLACK_BOT_TOKEN`). LLM API keys can be set as env vars or added after deploy via the admin UI.

Managed Postgres is included in the app spec. `DATABASE_URL` is auto-injected.

**Note:** The code-runner skill requires Docker socket access and is not available on DO App Platform. It works on Droplets and self-hosted deployments. All other skills work everywhere.

## Documentation

- [Getting Started](docs/getting-started.md)
- [Provider Routing](docs/provider-routing.md)
- [Skills Catalog](docs/skills-catalog.md)
- [Security Model](docs/security-model.md) (coming soon)

## License

See [LICENSE](LICENSE).
