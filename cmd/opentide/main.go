package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/opentide/opentide/internal/adapters"
	discordAdapter "github.com/opentide/opentide/internal/adapters/discord"
	slackAdapter "github.com/opentide/opentide/internal/adapters/slack"
	"github.com/opentide/opentide/internal/adapters/stdio"
	"github.com/opentide/opentide/internal/admin"
	"github.com/opentide/opentide/internal/approval"
	"github.com/opentide/opentide/internal/config"
	"github.com/opentide/opentide/internal/providers"
	"github.com/opentide/opentide/internal/security"
	anthropicProvider "github.com/opentide/opentide/internal/providers/anthropic"
	"github.com/opentide/opentide/internal/providers/gradient"
	openaiProvider "github.com/opentide/opentide/internal/providers/openai"
	"github.com/opentide/opentide/internal/skills"
	"github.com/opentide/opentide/internal/state"
	"github.com/opentide/opentide/internal/tenant"
	"github.com/opentide/opentide/pkg/skillspec"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	demo := flag.Bool("demo", false, "run in demo mode (stdio adapter, in-memory state)")
	dev := flag.Bool("dev", false, "run in dev mode (relaxed security, not for production)")
	flag.Parse()

	if *demo {
		os.Setenv("OPENTIDE_DEMO", "true")
	}
	if *dev {
		os.Setenv("OPENTIDE_DEV_MODE", "true")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	logger := setupLogger(cfg.Gateway.LogLevel)

	if cfg.Gateway.DevMode {
		logger.Warn("╔══════════════════════════════════════════════╗")
		logger.Warn("║  DEV MODE - NOT FOR PRODUCTION USE           ║")
		logger.Warn("║  Security controls are relaxed.              ║")
		logger.Warn("╚══════════════════════════════════════════════╝")
	}

	// Initialize LLM provider
	provider, err := initProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	logger.Info("LLM provider initialized", "provider", provider.Name(), "model", provider.ModelID())

	// Initialize state store
	var store state.Store
	if cfg.State.Driver == "postgres" && cfg.State.PostgresDSN != "" {
		pgStore, err := state.NewPostgresStore(context.Background(), cfg.State.PostgresDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Postgres error: %v\n", err)
			os.Exit(1)
		}
		store = pgStore
		defer pgStore.Close()
	} else {
		store = state.NewMemoryStore()
	}
	logger.Info("state store initialized", "driver", cfg.State.Driver)

	// Initialize approval engine
	ttl := time.Duration(cfg.Security.ApprovalTTL) * time.Second
	approvalEngine := approval.NewMemoryEngine(ttl, cfg.Gateway.DemoMode)
	logger.Info("approval engine initialized", "auto_approve", cfg.Gateway.DemoMode)

	// Initialize adapter
	var adapter adapters.Adapter
	if cfg.Gateway.DemoMode {
		adapter = stdio.New()
		logger.Info("running in demo mode (stdio adapter)")
	} else {
		var err error
		adapter, err = initAdapter(cfg, logger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Initialize skill engine
	var skillEngine skills.Engine
	if cfg.Gateway.DevMode {
		skillEngine = skills.NewProcessEngine()
	} else {
		skillEngine = skills.NewContainerEngine()
	}

	// Load skills from skills/ directory
	loadSkills(skillEngine, "skills", logger)

	// Initialize rate limiter
	rateLimiter := security.NewRateLimiter(security.DefaultRateLimitConfig())
	defer rateLimiter.Close()
	logger.Info("rate limiter initialized")

	// Initialize admin server
	tenantStore := tenant.NewMemoryStore()
	adminSrv := admin.NewServer(tenantStore, skillEngine, approvalEngine, rateLimiter, cfg, logger)

	// Determine admin bind address
	adminHost := cfg.Gateway.Host
	if cfg.Gateway.DemoMode {
		adminHost = "127.0.0.1" // demo mode binds localhost only
	}
	adminAddr := fmt.Sprintf("%s:%d", adminHost, cfg.Security.AdminPort)

	httpServer := &http.Server{
		Addr:              adminAddr,
		Handler:           adminSrv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start gateway and admin server concurrently
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Admin HTTP server
	g.Go(func() error {
		logger.Info("admin server starting", "addr", adminAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("admin server: %w", err)
		}
		return nil
	})

	// Graceful shutdown for admin server
	g.Go(func() error {
		<-ctx.Done()
		logger.Info("shutting down admin server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		return httpServer.Shutdown(shutdownCtx)
	})

	// Gateway (messaging adapters + LLM)
	gw := &Gateway{
		provider:    provider,
		adapter:     adapter,
		store:       store,
		approval:    approvalEngine,
		skills:      skillEngine,
		rateLimiter: rateLimiter,
		logger:      logger,
	}

	g.Go(func() error {
		return gw.Run(ctx)
	})

	if err := g.Wait(); err != nil {
		logger.Error("shutdown", "err", err)
		os.Exit(1)
	}
}

func setupLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

func initProvider(cfg *config.Config) (providers.Provider, error) {
	switch cfg.Providers.Default {
	case "anthropic":
		if cfg.Providers.Anthropic == nil || cfg.Providers.Anthropic.APIKey == "" {
			return nil, fmt.Errorf("anthropic provider selected but ANTHROPIC_API_KEY not set")
		}
		return anthropicProvider.New(cfg.Providers.Anthropic.APIKey, cfg.Providers.Anthropic.Model)
	case "openai":
		if cfg.Providers.OpenAI == nil || cfg.Providers.OpenAI.APIKey == "" {
			return nil, fmt.Errorf("openai provider selected but OPENAI_API_KEY not set")
		}
		return openaiProvider.New(cfg.Providers.OpenAI.APIKey, cfg.Providers.OpenAI.Model)
	case "gradient":
		if cfg.Providers.Gradient == nil || cfg.Providers.Gradient.APIKey == "" {
			return nil, fmt.Errorf("gradient provider selected but DO_GRADIENT_API_KEY not set")
		}
		return gradient.New(cfg.Providers.Gradient.APIKey, cfg.Providers.Gradient.Model, cfg.Providers.Gradient.BaseURL)
	default:
		// Auto-detect: try anthropic, then openai, then gradient
		if cfg.Providers.Anthropic != nil && cfg.Providers.Anthropic.APIKey != "" {
			return anthropicProvider.New(cfg.Providers.Anthropic.APIKey, cfg.Providers.Anthropic.Model)
		}
		if cfg.Providers.OpenAI != nil && cfg.Providers.OpenAI.APIKey != "" {
			return openaiProvider.New(cfg.Providers.OpenAI.APIKey, cfg.Providers.OpenAI.Model)
		}
		if cfg.Providers.Gradient != nil && cfg.Providers.Gradient.APIKey != "" {
			return gradient.New(cfg.Providers.Gradient.APIKey, cfg.Providers.Gradient.Model, cfg.Providers.Gradient.BaseURL)
		}
		return nil, fmt.Errorf("no LLM provider configured")
	}
}

func loadSkills(engine skills.Engine, dir string, logger *slog.Logger) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Debug("no skills directory found", "dir", dir)
		return
	}

	ctx := context.Background()
	loaded := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "skill.yaml")
		m, err := skillspec.LoadManifest(manifestPath)
		if err != nil {
			logger.Warn("skipping skill", "dir", entry.Name(), "err", err)
			continue
		}
		if err := engine.LoadSkill(ctx, m); err != nil {
			logger.Warn("failed to load skill", "name", m.Name, "err", err)
			continue
		}
		logger.Info("skill loaded", "name", m.Name, "version", m.Version, "tool", m.Triggers.ToolName)
		loaded++
	}
	if loaded > 0 {
		logger.Info("skills ready", "count", loaded)
	}
}

func initAdapter(cfg *config.Config, logger *slog.Logger) (adapters.Adapter, error) {
	// Prefer Discord, then Slack
	if cfg.Adapters.Discord != nil && cfg.Adapters.Discord.Token != "" {
		return discordAdapter.New(cfg.Adapters.Discord.Token, cfg.Adapters.Discord.GuildID, logger)
	}
	if cfg.Adapters.Slack != nil && cfg.Adapters.Slack.BotToken != "" {
		return slackAdapter.New(cfg.Adapters.Slack.BotToken, cfg.Adapters.Slack.AppToken, logger)
	}
	// TODO: add Telegram adapter
	return nil, fmt.Errorf("no adapter configured: set DISCORD_TOKEN, SLACK_BOT_TOKEN, or TELEGRAM_TOKEN")
}
