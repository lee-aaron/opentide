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
export ANTHROPIC_API_KEY=sk-ant-your-key  # or OPENAI_API_KEY, or DO_GRADIENT_API_KEY
go run ./cmd/opentide --demo
```

## Discord Bot

```bash
export ANTHROPIC_API_KEY=sk-ant-your-key
export DISCORD_TOKEN=your-bot-token
go run ./cmd/opentide
```

## Deploy to DigitalOcean

```bash
doctl apps create --spec deploy/app-platform/app.yaml
```

## Documentation

- [Getting Started](docs/getting-started.md)
- [Security Model](docs/security-model.md) (coming soon)

## License

See [LICENSE](LICENSE).
