// Package config handles loading and validating OpenTide configuration.
package config

import (
	"fmt"
	"os"
	"strings"

	oerr "github.com/opentide/opentide/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Config is the top-level OpenTide configuration.
type Config struct {
	Gateway   GatewayConfig   `yaml:"gateway"`
	Providers ProvidersConfig `yaml:"providers"`
	Adapters  AdaptersConfig  `yaml:"adapters"`
	State     StateConfig     `yaml:"state"`
	Security  SecurityConfig  `yaml:"security"`
}

type GatewayConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"log_level"`
	DemoMode bool   `yaml:"demo_mode"`
	DevMode  bool   `yaml:"dev_mode"`
}

type ProvidersConfig struct {
	Default   string          `yaml:"default"`
	Anthropic *AnthropicConfig `yaml:"anthropic,omitempty"`
	OpenAI    *OpenAIConfig    `yaml:"openai,omitempty"`
	Gradient  *GradientConfig  `yaml:"gradient,omitempty"`
	Routes    []RouteConfig    `yaml:"routes,omitempty"`
}

// RouteConfig is the YAML representation of a provider route.
type RouteConfig struct {
	ChannelID string `yaml:"channel_id,omitempty"`
	TenantID  string `yaml:"tenant_id,omitempty"`
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model,omitempty"`
	Priority  int    `yaml:"priority"`
}

type AnthropicConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type OpenAIConfig struct {
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url,omitempty"`
}

type GradientConfig struct {
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"`
}

type AdaptersConfig struct {
	Discord  *DiscordConfig  `yaml:"discord,omitempty"`
	Telegram *TelegramConfig `yaml:"telegram,omitempty"`
	Slack    *SlackConfig    `yaml:"slack,omitempty"`
}

type SlackConfig struct {
	BotToken string `yaml:"bot_token"` // xoxb-...
	AppToken string `yaml:"app_token"` // xapp-... (for Socket Mode)
}

type DiscordConfig struct {
	Token   string `yaml:"token"`
	GuildID string `yaml:"guild_id,omitempty"`
}

type TelegramConfig struct {
	Token      string `yaml:"token"`
	WebhookURL string `yaml:"webhook_url,omitempty"`
}

type StateConfig struct {
	Driver      string `yaml:"driver"` // "postgres" or "memory"
	PostgresDSN string `yaml:"postgres_dsn,omitempty"`
}

type SecurityConfig struct {
	MaxMessageSize int    `yaml:"max_message_size"` // bytes, default 65536
	ApprovalTTL    int    `yaml:"approval_ttl"`     // seconds, default 300
	AdminSecret    string `yaml:"admin_secret"`     // required in non-demo mode
	AdminPort      int    `yaml:"admin_port"`       // default 8080
	// Google OAuth for admin login (optional, falls back to admin secret)
	GoogleClientID     string `yaml:"google_client_id,omitempty"`
	GoogleClientSecret string `yaml:"google_client_secret,omitempty"`
	AdminEmails        string `yaml:"admin_emails,omitempty"` // comma-separated allowed emails
}

// Load reads config from a YAML file, with environment variable overrides.
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, oerr.New(oerr.CodeConfigMissing, fmt.Sprintf("cannot read config file: %s", path)).
				WithFix("Create a config file with 'tide-cli init' or specify a valid path with --config").
				WithDocs("https://github.com/opentide/opentide/blob/main/docs/getting-started.md")
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, oerr.Wrap(oerr.CodeConfigInvalid, "invalid YAML in config file", err).
				WithFix("Check your YAML syntax. Run 'tide-cli config validate' to find issues.")
		}
	}

	applyEnvOverrides(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Gateway: GatewayConfig{
			Host:     "0.0.0.0",
			Port:     8080,
			LogLevel: "info",
		},
		Providers: ProvidersConfig{
			Default: "anthropic",
		},
		State: StateConfig{
			Driver: "memory",
		},
		Security: SecurityConfig{
			MaxMessageSize: 65536,
			ApprovalTTL:    300,
			AdminPort:      8080,
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		if cfg.Providers.Anthropic == nil {
			cfg.Providers.Anthropic = &AnthropicConfig{}
		}
		cfg.Providers.Anthropic.APIKey = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		if cfg.Providers.OpenAI == nil {
			cfg.Providers.OpenAI = &OpenAIConfig{}
		}
		cfg.Providers.OpenAI.APIKey = v
	}
	// Support both MODEL_ACCESS_KEY (official DO name) and DO_GRADIENT_API_KEY (legacy)
	if v := os.Getenv("MODEL_ACCESS_KEY"); v != "" {
		if cfg.Providers.Gradient == nil {
			cfg.Providers.Gradient = &GradientConfig{}
		}
		cfg.Providers.Gradient.APIKey = v
	}
	if v := os.Getenv("DO_GRADIENT_API_KEY"); v != "" {
		if cfg.Providers.Gradient == nil {
			cfg.Providers.Gradient = &GradientConfig{}
		}
		cfg.Providers.Gradient.APIKey = v
	}
	if v := os.Getenv("GRADIENT_INFERENCE_ENDPOINT"); v != "" {
		if cfg.Providers.Gradient == nil {
			cfg.Providers.Gradient = &GradientConfig{}
		}
		cfg.Providers.Gradient.BaseURL = v
	}
	if v := os.Getenv("DISCORD_TOKEN"); v != "" {
		if cfg.Adapters.Discord == nil {
			cfg.Adapters.Discord = &DiscordConfig{}
		}
		cfg.Adapters.Discord.Token = v
	}
	if v := os.Getenv("TELEGRAM_TOKEN"); v != "" {
		if cfg.Adapters.Telegram == nil {
			cfg.Adapters.Telegram = &TelegramConfig{}
		}
		cfg.Adapters.Telegram.Token = v
	}
	if v := os.Getenv("SLACK_BOT_TOKEN"); v != "" {
		if cfg.Adapters.Slack == nil {
			cfg.Adapters.Slack = &SlackConfig{}
		}
		cfg.Adapters.Slack.BotToken = v
	}
	if v := os.Getenv("SLACK_APP_TOKEN"); v != "" {
		if cfg.Adapters.Slack == nil {
			cfg.Adapters.Slack = &SlackConfig{}
		}
		cfg.Adapters.Slack.AppToken = v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.State.Driver = "postgres"
		cfg.State.PostgresDSN = v
	}
	if strings.EqualFold(os.Getenv("OPENTIDE_DEMO"), "true") {
		cfg.Gateway.DemoMode = true
		cfg.State.Driver = "memory"
	}
	if strings.EqualFold(os.Getenv("OPENTIDE_DEV_MODE"), "true") {
		cfg.Gateway.DevMode = true
	}
	if v := os.Getenv("OPENTIDE_ADMIN_SECRET"); v != "" {
		cfg.Security.AdminSecret = v
	}
	if v := os.Getenv("GOOGLE_CLIENT_ID"); v != "" {
		cfg.Security.GoogleClientID = v
	}
	if v := os.Getenv("GOOGLE_CLIENT_SECRET"); v != "" {
		cfg.Security.GoogleClientSecret = v
	}
	if v := os.Getenv("OPENTIDE_ADMIN_EMAILS"); v != "" {
		cfg.Security.AdminEmails = v
	}
}

func validate(cfg *Config) error {
	// In demo mode, we need either an LLM provider key or an admin secret (to add keys via UI)
	if cfg.Gateway.DemoMode {
		if !hasAnyProvider(cfg) && cfg.Security.AdminSecret == "" && !hasGoogleOAuth(cfg) {
			return oerr.New(oerr.CodeConfigEnvEmpty, "demo mode requires an LLM provider key or admin authentication").
				WithFix("Set ANTHROPIC_API_KEY (or another provider key), or set OPENTIDE_ADMIN_SECRET, or configure Google OAuth").
				WithDocs("https://github.com/opentide/opentide/blob/main/docs/getting-started.md")
		}
		return nil
	}

	hasAdminAuth := cfg.Security.AdminSecret != "" || hasGoogleOAuth(cfg)

	// Full mode: allow starting without provider keys if admin auth is configured (keys can be added via UI)
	if !hasAnyProvider(cfg) && !hasAdminAuth {
		return oerr.New(oerr.CodeConfigEnvEmpty, "no LLM provider configured").
			WithFix("Set at least one of: ANTHROPIC_API_KEY, OPENAI_API_KEY, or MODEL_ACCESS_KEY. Or set OPENTIDE_ADMIN_SECRET (or GOOGLE_CLIENT_ID + GOOGLE_CLIENT_SECRET) to add keys via the admin UI.").
			WithDocs("https://github.com/opentide/opentide/blob/main/docs/getting-started.md")
	}

	if !hasAdminAuth {
		return oerr.New(oerr.CodeConfigEnvEmpty, "admin authentication is required in non-demo mode").
			WithFix("Set OPENTIDE_ADMIN_SECRET or configure Google OAuth (GOOGLE_CLIENT_ID + GOOGLE_CLIENT_SECRET)").
			WithDocs("https://github.com/opentide/opentide/blob/main/docs/admin-api.md")
	}

	if cfg.Adapters.Discord == nil && cfg.Adapters.Telegram == nil && cfg.Adapters.Slack == nil {
		// Allow starting without adapters if admin auth is configured (admin UI only mode)
		if hasAdminAuth {
			return nil
		}
		return oerr.New(oerr.CodeConfigEnvEmpty, "no messaging adapter configured").
			WithFix("Set DISCORD_TOKEN, TELEGRAM_TOKEN, or SLACK_BOT_TOKEN environment variable").
			WithDocs("https://github.com/opentide/opentide/blob/main/docs/getting-started.md")
	}

	return nil
}

func hasAnyProvider(cfg *Config) bool {
	return (cfg.Providers.Anthropic != nil && cfg.Providers.Anthropic.APIKey != "") ||
		(cfg.Providers.OpenAI != nil && cfg.Providers.OpenAI.APIKey != "") ||
		(cfg.Providers.Gradient != nil && cfg.Providers.Gradient.APIKey != "")
}

func hasGoogleOAuth(cfg *Config) bool {
	return cfg.Security.GoogleClientID != "" && cfg.Security.GoogleClientSecret != ""
}
