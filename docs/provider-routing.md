# Provider Routing

OpenTide supports multiple LLM providers simultaneously. You can route messages to different providers based on channel, tenant, or user preference.

## Configuration

Configure multiple providers in `opentide.yaml`:

```yaml
providers:
  default: anthropic
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-sonnet-4-20250514
  openai:
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o
  gradient:
    api_key: ${MODEL_ACCESS_KEY}
    model: meta-llama/Llama-3.3-70B-Instruct
```

## Route Rules

Routes send messages from specific channels to specific providers:

```yaml
providers:
  routes:
    - channel_id: "engineering"
      provider: anthropic
      model: claude-sonnet-4-20250514
      priority: 10
    - channel_id: "general"
      provider: openai
      model: gpt-4o
      priority: 5
    - channel_id: "*"
      provider: gradient
      priority: 1
```

Higher priority routes match first. Wildcard (`*`) matches any channel.

## Resolution Order

When a message arrives, OpenTide resolves the provider in this order:

1. **User override** (set via `/model` command, expires after 24h of inactivity)
2. **Channel route** (highest priority matching route)
3. **Global default** (from `providers.default` config)

The resolved provider is pinned for the entire message handling, including all tool calls. No switching mid-conversation-turn.

## Runtime Switching

Users can switch providers in chat:

```
/model                     Show current provider and available options
/model anthropic           Switch to Anthropic (default model)
/model openai gpt-4o       Switch to OpenAI with specific model
/model gradient            Switch to DO Gradient
/model reset               Clear override, use channel/default routing
```

Overrides persist for 24 hours since last message. Cleared automatically or via `/model reset`.

## Per-Route Security Policies

Routes can include security constraints:

```yaml
providers:
  routes:
    - channel_id: "engineering"
      provider: anthropic
      priority: 10
      security:
        max_tokens_per_request: 4096
        audit_verbosity: full
```

- `max_tokens_per_request`: Cap token usage per request (0 = unlimited)
- `audit_verbosity`: `minimal`, `standard`, or `full` audit logging

## Admin API

Manage routes via the admin API:

```bash
# List all routes
curl -b cookies.txt http://localhost:8080/admin/api/providers/routes

# Create a route
curl -b cookies.txt -X POST http://localhost:8080/admin/api/providers/routes \
  -H 'Content-Type: application/json' \
  -d '{"channel_id":"engineering","provider":"anthropic","priority":10}'

# Test route resolution
curl -b cookies.txt -X POST http://localhost:8080/admin/api/providers/test-route \
  -H 'Content-Type: application/json' \
  -d '{"channel_id":"engineering","user_id":"user123"}'

# Delete a route
curl -b cookies.txt -X DELETE http://localhost:8080/admin/api/providers/routes/engineering
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `MODEL_ACCESS_KEY` | DO Gradient model access key |
| `OPENTIDE_DEFAULT_PROVIDER` | Override default provider (anthropic, openai, gradient) |
