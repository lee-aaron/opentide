package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Set minimal env for demo mode
	os.Setenv("OPENTIDE_DEMO", "true")
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("OPENTIDE_DEMO")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Gateway.DemoMode {
		t.Fatal("expected demo mode")
	}
	if cfg.State.Driver != "memory" {
		t.Fatalf("expected memory driver in demo mode, got: %s", cfg.State.Driver)
	}
	if cfg.Security.MaxMessageSize != 65536 {
		t.Fatalf("expected default max message size, got: %d", cfg.Security.MaxMessageSize)
	}
}

func TestLoad_MissingProviderInDemo(t *testing.T) {
	os.Setenv("OPENTIDE_DEMO", "true")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("DO_GRADIENT_API_KEY")
	defer os.Unsetenv("OPENTIDE_DEMO")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for missing provider in demo mode")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	os.Setenv("DISCORD_TOKEN", "disc-tok")
	os.Setenv("OPENTIDE_ADMIN_SECRET", "test-secret")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("DISCORD_TOKEN")
	defer os.Unsetenv("OPENTIDE_ADMIN_SECRET")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Providers.Anthropic == nil || cfg.Providers.Anthropic.APIKey != "sk-ant-test" {
		t.Fatal("expected Anthropic API key from env")
	}
	if cfg.Adapters.Discord == nil || cfg.Adapters.Discord.Token != "disc-tok" {
		t.Fatal("expected Discord token from env")
	}
}

func TestLoad_NoAdapterInFullMode(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	os.Setenv("OPENTIDE_ADMIN_SECRET", "test-secret")
	os.Unsetenv("DISCORD_TOKEN")
	os.Unsetenv("TELEGRAM_TOKEN")
	os.Unsetenv("SLACK_BOT_TOKEN")
	os.Unsetenv("OPENTIDE_DEMO")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("OPENTIDE_ADMIN_SECRET")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for missing adapter in full mode")
	}
}

func TestLoad_NoAdminSecretInFullMode(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	os.Setenv("DISCORD_TOKEN", "disc-tok")
	os.Unsetenv("OPENTIDE_ADMIN_SECRET")
	os.Unsetenv("OPENTIDE_DEMO")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("DISCORD_TOKEN")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for missing admin secret in full mode")
	}
}
