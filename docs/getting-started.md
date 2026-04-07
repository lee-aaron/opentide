# Getting Started with OpenTide

## Quick Start (Demo Mode, 2 minutes)

You need Go 1.21+.

```bash
# Clone the repo
git clone https://github.com/opentide/opentide.git
cd opentide

# Set admin secret (required)
export OPENTIDE_ADMIN_SECRET=$(go run ./cmd/tide-cli admin secret 2>/dev/null)

# Option A: Set an API key now
export ANTHROPIC_API_KEY=sk-ant-your-key-here

# Option B: Skip API keys, add them via admin UI after startup
# (just set OPENTIDE_ADMIN_SECRET above)

# Run in demo mode (interactive CLI, no external dependencies)
go run ./cmd/opentide --demo
```

If you set an API key, you'll get a chat prompt immediately. If not, open `http://localhost:8080/admin/` to add keys through the dashboard.

## Discord Bot Setup

### 1. Create the Application

Go to https://discord.com/developers/applications and click **New Application**. Give it a name (e.g., "OpenTide").

### 2. Get the Bot Token

Go to the **Bot** tab. Click **Reset Token** and copy it. This is your `DISCORD_TOKEN`.

Under **Privileged Gateway Intents**, enable:
- **Message Content Intent** (required - the bot reads message text)

Leave Server Members and Presence intents off.

### 3. Invite the Bot to Your Server

Go to **OAuth2 > URL Generator**.

Select scopes:
- `bot`
- `applications.commands`

Select bot permissions:
- Send Messages
- Read Message History
- Use Slash Commands

Copy the generated URL and open it in your browser. Select your server and authorize.

### 4. Start the Bot

```bash
export OPENTIDE_ADMIN_SECRET=your-admin-secret
export DISCORD_TOKEN=your-bot-token
export ANTHROPIC_API_KEY=sk-ant-your-key  # or add via admin UI

go run ./cmd/opentide
```

### 5. Talk to the Bot

**In server channels:** @mention the bot (e.g., `@OpenTide what's the weather?`) or use the `/chat` slash command.

**In DMs:** Click the bot's name in the server member list, then "Message". No @mention needed in DMs, just type directly.

The bot also supports approval buttons for skill actions that require user confirmation.

## Configuration

Initialize a config file:

```bash
go run ./cmd/tide-cli init
```

This creates `opentide.yaml`. Edit it or use environment variables (env vars override the file).

## LLM Providers

OpenTide supports three providers. Set API keys via environment variables or add them through the admin UI at `/admin/` (Providers page).

| Provider | Env Variable | Models |
|----------|-------------|--------|
| Anthropic | `ANTHROPIC_API_KEY` | claude-sonnet-4-20250514 (default) |
| OpenAI | `OPENAI_API_KEY` | gpt-4o (default) |
| DO Gradient | `MODEL_ACCESS_KEY` | meta-llama/Llama-3.3-70B-Instruct (default) |

`DO_GRADIENT_API_KEY` is also accepted as a silent fallback for the Gradient provider.

**API keys added through the admin UI** are encrypted at rest with AES-256-GCM (key derived from your `OPENTIDE_ADMIN_SECRET`). Environment variables always take precedence over UI-stored keys.

## Multi-Provider Setup

OpenTide supports multiple LLM providers simultaneously. Configure all providers you want to use:

```bash
export ANTHROPIC_API_KEY=sk-ant-your-key-here
export OPENAI_API_KEY=sk-your-key-here
export MODEL_ACCESS_KEY=your-do-gradient-key

go run ./cmd/opentide --demo
```

Users can switch providers mid-conversation with `/model`:

```
/model                  → show current provider
/model openai gpt-4o    → switch to OpenAI
/model anthropic        → switch to Anthropic
/model reset            → use default routing
```

For channel-based routing (e.g., #engineering uses Claude, #general uses GPT-4o), see [Provider Routing](provider-routing.md).

## Admin Dashboard

OpenTide includes a security-focused admin UI at `http://localhost:8080/admin/`.

### Setup

```bash
# Generate an admin secret
go run ./cmd/tide-cli admin secret
# Set it as an env var
export OPENTIDE_ADMIN_SECRET=<the-generated-secret>

# Start the gateway
go run ./cmd/opentide
```

Open `http://localhost:8080/admin/` in your browser and log in with your admin secret.

In demo mode, auth is bypassed and the dashboard binds to `127.0.0.1` only.

### What you get

- **Security Dashboard** - live feed of approval decisions, TOCTOU mismatch alerts, egress violations
- **Audit Log** - full history with side-by-side payload diffs for mismatch investigation
- **Egress Posture** - per-skill network permissions and violation log
- **Provider Routing** - manage routes, test resolution, view provider health
- **Approval Policies** - configure auto-approve rules with scope limits
- **Tenant & Skill Management** - CRUD for tenants, skill status and config

## Skills

OpenTide ships with 10 built-in skills. List them:

```bash
go run ./cmd/tide-cli skill list
```

### Scaffold a new skill

```bash
go run ./cmd/tide-cli skill new my-skill
```

This creates `skills/my-skill/` with a complete starter template: `skill.yaml`, `main.go`, `go.mod`, and `Dockerfile`. Edit the manifest to declare your security constraints and implement your logic in `main.go`.

Skills communicate via stdin/stdout JSON. Input: `{"arguments":{"query":"..."}}`. Output: `{"content":"..."}` or `{"error":"..."}`.

See the full [Skills Catalog](skills-catalog.md) for all 10 skills with security postures.

## CLI Reference

```bash
tide-cli init               # Create opentide.yaml in current directory
tide-cli run [--demo]       # Start the gateway
tide-cli status             # Show config and connection status
tide-cli config validate    # Validate opentide.yaml

tide-cli skill new <name>   # Scaffold a new skill
tide-cli skill list         # List installed skills
tide-cli skill sign ...     # Sign a skill manifest
tide-cli skill verify ...   # Verify a signed manifest
tide-cli skill keygen       # Generate Ed25519 signing key pair
tide-cli skill publish ...  # Publish to registry
tide-cli skill search ...   # Search the registry
tide-cli skill install ...  # Install from registry

tide-cli admin secret       # Generate a random 32-byte admin secret
tide-cli version            # Show version
```

## Deploy to DigitalOcean

### App Platform

```bash
doctl apps create --spec deploy/app-platform/app.yaml
```

Then set these env vars in the DO App Platform dashboard:

| Variable | Required | Notes |
|----------|----------|-------|
| `OPENTIDE_ADMIN_SECRET` | Yes | Generate with `tide-cli admin secret` |
| `DISCORD_TOKEN` | Yes (or Slack) | Messaging adapter |
| `SLACK_BOT_TOKEN` | Yes (or Discord) | Messaging adapter |
| `ANTHROPIC_API_KEY` | Optional | Can be added via admin UI instead |
| `OPENAI_API_KEY` | Optional | Can be added via admin UI instead. Also used by image-gen skill |
| `MODEL_ACCESS_KEY` | Optional | Can be added via admin UI instead. DO Control Panel → Serverless Inference |
| `GITHUB_TOKEN` | Optional | For github skill (private repos, higher rate limits) |
| `BRAVE_API_KEY` | Optional | For web-search skill |

`DATABASE_URL` is auto-injected from the managed Postgres database in the app spec.

LLM API keys can be set as env vars or added after deployment through the admin UI at `/admin/` (Providers page). Keys added via UI are encrypted at rest.

**Limitation:** The code-runner skill requires Docker socket access and is **not available** on DO App Platform. It auto-detects and skips gracefully. All other skills work. For code-runner, deploy on a Droplet or self-hosted environment.

### Docker

```bash
docker build -f deploy/docker/Dockerfile -t opentide .
docker run \
  -e ANTHROPIC_API_KEY=xxx \
  -e DISCORD_TOKEN=xxx \
  -e OPENTIDE_ADMIN_SECRET=xxx \
  -p 8080:8080 \
  opentide
```

### Droplet (with code-runner)

On a Droplet with Docker installed, all 10 skills work including code-runner:

```bash
docker build -f deploy/docker/Dockerfile -t opentide .
docker run \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e ANTHROPIC_API_KEY=xxx \
  -e DISCORD_TOKEN=xxx \
  -e OPENTIDE_ADMIN_SECRET=xxx \
  -p 8080:8080 \
  opentide
```

## Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `[PROVIDER_AUTH] failed to initialize Anthropic provider` | Bad or missing API key | Check `ANTHROPIC_API_KEY` is set and valid |
| `[PROVIDER_AUTH] no LLM provider configured` | No API keys set | Set at least one of `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `MODEL_ACCESS_KEY` |
| `[ADAPTER_CONNECT] failed to connect to Discord gateway` | Bad bot token or missing intents | Check token, enable Message Content Intent |
| `skipping skill: platform requirement not met` | Docker not available | Expected on App Platform for code-runner. Deploy on Droplet for full skill set. |

## Dev Mode

For local development with relaxed security:

```bash
go run ./cmd/opentide --dev --demo
```

Dev mode disables strict egress controls and uses process-level isolation instead of containers. **Never use in production.**
