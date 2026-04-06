package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/opentide/opentide/internal/config"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		cmdInit()
	case "run":
		cmdRun()
	case "status":
		cmdStatus()
	case "version":
		fmt.Printf("tide-cli v%s\n", version)
	case "config":
		if len(os.Args) > 2 && os.Args[2] == "validate" {
			cmdConfigValidate()
		} else {
			printUsage()
		}
	case "skill":
		cmdSkill()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`tide-cli - OpenTide management tool

Usage: tide-cli <command>

Commands:
  init              Initialize a new OpenTide project in the current directory
  run               Start the OpenTide gateway (shortcut for 'go run ./cmd/opentide')
  status            Show current configuration and connection status
  config validate   Validate the configuration file
  skill list        List skills found in the skills/ directory
  skill verify      Verify a signed skill manifest
  skill sign        Sign a skill manifest (requires private key)
  skill keygen      Generate an Ed25519 signing key pair
  skill publish     Publish a signed skill to the registry
  skill search      Search the skill registry
  skill install     Install a skill from the registry
  version           Show version

Flags:
  --demo            Run in demo mode (stdio adapter, in-memory state)
  --dev             Run in dev mode (relaxed security)

Examples:
  tide-cli init
  ANTHROPIC_API_KEY=sk-ant-xxx tide-cli run --demo
  tide-cli skill list
  tide-cli skill sign skills/web-search/skill.yaml --key signing.key
  tide-cli config validate`)
}

func cmdInit() {
	configPath := "opentide.yaml"

	if _, err := os.Stat(configPath); err == nil {
		fmt.Println("opentide.yaml already exists. Skipping.")
		return
	}

	cfg := config.Config{
		Gateway: config.GatewayConfig{
			Host:     "0.0.0.0",
			Port:     8080,
			LogLevel: "info",
		},
		Providers: config.ProvidersConfig{
			Default: "anthropic",
		},
		Adapters: config.AdaptersConfig{},
		State: config.StateConfig{
			Driver: "memory",
		},
		Security: config.SecurityConfig{
			MaxMessageSize: 65536,
			ApprovalTTL:    300,
		},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating config: %v\n", err)
		os.Exit(1)
	}

	header := `# OpenTide Configuration
# See: https://github.com/opentide/opentide/blob/main/docs/getting-started.md
#
# Set your API keys as environment variables:
#   ANTHROPIC_API_KEY=sk-ant-xxx
#   OPENAI_API_KEY=sk-xxx
#   DO_GRADIENT_API_KEY=xxx
#   DISCORD_TOKEN=xxx
#   DATABASE_URL=postgres://...
#
`
	if err := os.WriteFile(configPath, []byte(header+string(data)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Created opentide.yaml")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  1. Set your API key:  export ANTHROPIC_API_KEY=sk-ant-xxx")
	fmt.Println("  2. Try demo mode:     tide-cli run --demo")
	fmt.Println("  3. Or with Discord:   export DISCORD_TOKEN=xxx && tide-cli run")
}

func cmdRun() {
	// Find the opentide binary or use go run
	binary, err := exec.LookPath("opentide")
	if err != nil {
		// Fall back to go run
		binary = "go"
	}

	args := []string{}
	if binary == "go" {
		args = append(args, "run", "./cmd/opentide")
	}

	// Pass through flags
	configPath := ""
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--config" && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
			i++
		}
		args = append(args, arg)
	}

	// Auto-detect config file
	if configPath == "" {
		if _, err := os.Stat("opentide.yaml"); err == nil {
			args = append(args, "--config", "opentide.yaml")
		}
	}

	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdStatus() {
	configPath := "opentide.yaml"
	if len(os.Args) > 2 {
		configPath = os.Args[2]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("Config:    ERROR (%v)\n", err)
		return
	}

	fmt.Println("OpenTide Status")
	fmt.Println("───────────────")
	fmt.Printf("Config:    %s\n", filepath.Clean(configPath))
	fmt.Printf("Mode:      %s\n", modeString(cfg))
	fmt.Printf("Provider:  %s\n", cfg.Providers.Default)
	fmt.Printf("State:     %s\n", cfg.State.Driver)

	// Check providers
	fmt.Println("\nProviders:")
	if cfg.Providers.Anthropic != nil && cfg.Providers.Anthropic.APIKey != "" {
		fmt.Println("  Anthropic:  configured")
	}
	if cfg.Providers.OpenAI != nil && cfg.Providers.OpenAI.APIKey != "" {
		fmt.Println("  OpenAI:     configured")
	}
	if cfg.Providers.Gradient != nil && cfg.Providers.Gradient.APIKey != "" {
		fmt.Println("  Gradient:   configured")
	}

	// Check adapters
	fmt.Println("\nAdapters:")
	if cfg.Adapters.Discord != nil && cfg.Adapters.Discord.Token != "" {
		fmt.Println("  Discord:    configured")
	}
	if cfg.Adapters.Telegram != nil && cfg.Adapters.Telegram.Token != "" {
		fmt.Println("  Telegram:   configured")
	}
	if cfg.Adapters.Slack != nil && cfg.Adapters.Slack.BotToken != "" {
		fmt.Println("  Slack:      configured")
	}
	if cfg.Gateway.DemoMode {
		fmt.Println("  Stdio:      active (demo mode)")
	}
}

func cmdConfigValidate() {
	configPath := "opentide.yaml"
	if len(os.Args) > 3 {
		configPath = os.Args[3]
	}

	_, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "INVALID: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Config is valid.")
}

func modeString(cfg *config.Config) string {
	if cfg.Gateway.DemoMode {
		return "demo"
	}
	if cfg.Gateway.DevMode {
		return "dev (NOT FOR PRODUCTION)"
	}
	return "production"
}
