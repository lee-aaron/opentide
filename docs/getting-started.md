# Getting Started with OpenTide

## Quick Start (Demo Mode, 2 minutes)

You need Go 1.21+ and one LLM API key.

```bash
# Clone the repo
git clone https://github.com/opentide/opentide.git
cd opentide

# Set your API key (pick one)
export ANTHROPIC_API_KEY=sk-ant-your-key-here
# or: export OPENAI_API_KEY=sk-your-key-here
# or: export DO_GRADIENT_API_KEY=your-key-here

# Run in demo mode (interactive CLI, no external dependencies)
go run ./cmd/opentide --demo
```

You'll see a prompt. Type a message and get a response from your chosen LLM.

## Discord Bot Setup

1. Create a Discord application at https://discord.com/developers/applications
2. Go to Bot settings, click "Add Bot"
3. Enable "Message Content Intent" under Privileged Gateway Intents
4. Copy the bot token
5. Invite the bot to your server using the OAuth2 URL Generator (scopes: `bot`, `applications.commands`)

```bash
export ANTHROPIC_API_KEY=sk-ant-your-key-here
export DISCORD_TOKEN=your-bot-token

go run ./cmd/opentide
```

Mention the bot in any channel or use the `/chat` slash command.

## Configuration

Initialize a config file:

```bash
go run ./cmd/tide-cli init
```

This creates `opentide.yaml`. Edit it or use environment variables (env vars override the file).

## LLM Providers

OpenTide supports three providers. Set the default in config or pick whichever API key you have:

| Provider | Env Variable | Models |
|----------|-------------|--------|
| Anthropic | `ANTHROPIC_API_KEY` | claude-sonnet-4-20250514 (default) |
| OpenAI | `OPENAI_API_KEY` | gpt-4o (default) |
| DO Gradient | `DO_GRADIENT_API_KEY` | meta-llama/Llama-3.3-70B-Instruct (default) |

## Deploy to DigitalOcean

```bash
# Using DO App Platform
doctl apps create --spec deploy/app-platform/app.yaml

# Using Docker
docker build -f deploy/docker/Dockerfile -t opentide .
docker run -e ANTHROPIC_API_KEY=xxx -e DISCORD_TOKEN=xxx opentide
```

## Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `[PROVIDER_AUTH] API key is empty` | Missing API key | Set the appropriate env variable |
| `[CONFIG_ENV_EMPTY] no messaging adapter configured` | No Discord/Telegram token | Set `DISCORD_TOKEN` or use `--demo` |
| `[ADAPTER_CONNECT] failed to connect to Discord gateway` | Bad bot token or missing intents | Check token, enable Message Content Intent |

## Dev Mode

For local development with relaxed security:

```bash
go run ./cmd/opentide --dev --demo
```

Dev mode disables strict egress controls and uses process-level isolation instead of containers. **Never use in production.**
