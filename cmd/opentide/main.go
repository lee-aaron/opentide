package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opentide/opentide/internal/adapters"
	discordAdapter "github.com/opentide/opentide/internal/adapters/discord"
	slackAdapter "github.com/opentide/opentide/internal/adapters/slack"
	"github.com/opentide/opentide/internal/adapters/stdio"
	"github.com/opentide/opentide/internal/admin"
	"github.com/opentide/opentide/internal/approval"
	"github.com/opentide/opentide/internal/config"
	"github.com/opentide/opentide/internal/memory"
	"github.com/opentide/opentide/internal/providers"
	"github.com/opentide/opentide/internal/security"
	"github.com/opentide/opentide/internal/security/secrets"
	anthropicProvider "github.com/opentide/opentide/internal/providers/anthropic"
	"github.com/opentide/opentide/internal/providers/gradient"
	openaiProvider "github.com/opentide/opentide/internal/providers/openai"
	"github.com/opentide/opentide/internal/skills"
	"github.com/opentide/opentide/internal/state"
	"github.com/opentide/opentide/internal/tenant"
	oerr "github.com/opentide/opentide/pkg/errors"
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

	// Initialize Postgres pool (shared across state store and secrets store)
	var pgPool *pgxpool.Pool
	if cfg.State.Driver == "postgres" && cfg.State.PostgresDSN != "" {
		pool, err := state.ConnectPool(context.Background(), cfg.State.PostgresDSN, logger)
		if err != nil {
			logger.Warn("postgres unavailable, falling back to in-memory state",
				"err", err,
				"fix", "use a production database cluster or run 'tide-cli db migrate' with a privileged user")
		} else {
			pgPool = pool
			defer pool.Close()
		}
	}

	// Initialize encrypted secrets store for API keys
	var secretStore secrets.Store
	if cfg.Security.AdminSecret != "" {
		encKey, err := secrets.DeriveKey(cfg.Security.AdminSecret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deriving secrets key: %v\n", err)
			os.Exit(1)
		}
		if pgPool != nil {
			secretStore = secrets.NewPostgresStore(pgPool, encKey)
			logger.Info("secrets store initialized (encrypted, postgres)")
		} else {
			secretStore = secrets.NewMemoryStore(encKey)
			logger.Info("secrets store initialized (encrypted, in-memory)")
		}
	}

	// Initialize LLM providers (multi-provider registry)
	registry, err := initProviders(cfg, secretStore, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defaultProvider := registry.Default()
	if defaultProvider != nil {
		logger.Info("LLM providers initialized", "default", defaultProvider.Name(), "model", defaultProvider.ModelID())
	}
	for _, info := range registry.List() {
		logger.Info("provider registered", "name", info.Name, "model", info.Model)
	}

	// Initialize state store
	var store state.Store
	if pgPool != nil {
		store = state.NewPostgresStore(pgPool)
	} else {
		store = state.NewMemoryStore()
	}
	logger.Info("state store initialized", "driver", cfg.State.Driver)

	// Initialize user memory store
	var memoryStore memory.Store
	if pgPool != nil {
		memoryStore = memory.NewPostgresStore(pgPool)
	} else {
		memoryStore = memory.NewMemoryStore()
	}
	logger.Info("user memory store initialized")

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
			// If no adapter but admin secret is set, run in admin-only mode
			if cfg.Security.AdminSecret != "" {
				logger.Warn("no messaging adapter configured, running admin UI only")
				logger.Warn("add DISCORD_TOKEN or SLACK_BOT_TOKEN to enable messaging")
			} else {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
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
	adminSrv := admin.NewServer(tenantStore, skillEngine, approvalEngine, rateLimiter, registry, secretStore, cfg, logger)

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

	// Start registry cleanup goroutine (reaps expired user overrides)
	registry.StartCleanup(ctx)

	// Gateway (messaging adapters + LLM) — only start if an adapter is configured
	if adapter != nil {
		gw := &Gateway{
			registry:    registry,
			adapter:     adapter,
			store:       store,
			memory:      memoryStore,
			approval:    approvalEngine,
			skills:      skillEngine,
			rateLimiter: rateLimiter,
			logger:      logger,
		}

		g.Go(func() error {
			return gw.Run(ctx)
		})
	} else {
		logger.Info("admin-only mode: admin UI available at http://" + adminAddr + "/admin/")
	}

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

func initProviders(cfg *config.Config, secretStore secrets.Store, logger *slog.Logger) (*providers.Registry, error) {
	// Convert config routes to provider routes.
	var routes []providers.Route
	for _, rc := range cfg.Providers.Routes {
		routes = append(routes, providers.Route{
			ChannelID: rc.ChannelID,
			TenantID:  rc.TenantID,
			Provider:  rc.Provider,
			Model:     rc.Model,
			Priority:  rc.Priority,
		})
	}

	fallback := cfg.Providers.Default
	registry := providers.NewRegistry(fallback, routes, logger)

	// Register all configured providers.
	if cfg.Providers.Anthropic != nil && cfg.Providers.Anthropic.APIKey != "" {
		p, err := anthropicProvider.New(cfg.Providers.Anthropic.APIKey, cfg.Providers.Anthropic.Model)
		if err != nil {
			return nil, oerr.Wrap(oerr.CodeProviderAuth, "failed to initialize Anthropic provider", err).
				WithFix("Check that ANTHROPIC_API_KEY is valid").
				WithDocs("https://docs.anthropic.com/en/api/getting-started")
		}
		registry.Register("anthropic", p)
	}

	if cfg.Providers.OpenAI != nil && cfg.Providers.OpenAI.APIKey != "" {
		p, err := openaiProvider.New(cfg.Providers.OpenAI.APIKey, cfg.Providers.OpenAI.Model)
		if err != nil {
			return nil, oerr.Wrap(oerr.CodeProviderAuth, "failed to initialize OpenAI provider", err).
				WithFix("Check that OPENAI_API_KEY is valid")
		}
		registry.Register("openai", p)
	}

	if cfg.Providers.Gradient != nil && cfg.Providers.Gradient.APIKey != "" {
		p, err := gradient.New(cfg.Providers.Gradient.APIKey, cfg.Providers.Gradient.Model, cfg.Providers.Gradient.BaseURL)
		if err != nil {
			return nil, oerr.Wrap(oerr.CodeProviderAuth, "failed to initialize DO Gradient provider", err).
				WithFix("Check that MODEL_ACCESS_KEY is valid (DO Control Panel → Serverless Inference → Model Access Keys)")
		}
		registry.Register("gradient", p)
	}

	// Check secrets store for providers not yet registered via env vars.
	if secretStore != nil {
		ctx := context.Background()
		for _, name := range []string{"anthropic", "openai", "gradient"} {
			if _, ok := registry.Get(name); ok {
				continue // already configured via env
			}
			apiKey, err := secretStore.Get(ctx, name)
			if err != nil || apiKey == "" {
				continue
			}
			var p providers.Provider
			switch name {
			case "anthropic":
				model := "claude-sonnet-4-20250514"
				if cfg.Providers.Anthropic != nil && cfg.Providers.Anthropic.Model != "" {
					model = cfg.Providers.Anthropic.Model
				}
				p, err = anthropicProvider.New(apiKey, model)
			case "openai":
				model := "gpt-4o"
				if cfg.Providers.OpenAI != nil && cfg.Providers.OpenAI.Model != "" {
					model = cfg.Providers.OpenAI.Model
				}
				p, err = openaiProvider.New(apiKey, model)
			case "gradient":
				model := "meta-llama/Llama-3.3-70B-Instruct"
				baseURL := ""
				if cfg.Providers.Gradient != nil {
					if cfg.Providers.Gradient.Model != "" {
						model = cfg.Providers.Gradient.Model
					}
					baseURL = cfg.Providers.Gradient.BaseURL
				}
				p, err = gradient.New(apiKey, model, baseURL)
			}
			if err != nil {
				logger.Warn("stored secret failed to initialize provider", "provider", name, "err", err)
				continue
			}
			registry.Register(name, p)
			logger.Info("provider loaded from stored secret", "provider", name)
		}
	}

	// Allow starting with no providers if admin secret is set (keys can be added via UI).
	if len(registry.List()) == 0 && cfg.Security.AdminSecret == "" {
		return nil, oerr.New(oerr.CodeProviderAuth, "no LLM provider configured").
			WithFix("Set at least one of: ANTHROPIC_API_KEY, OPENAI_API_KEY, or MODEL_ACCESS_KEY. Or set OPENTIDE_ADMIN_SECRET to add keys via the admin UI.")
	}

	// If no explicit default or default not found, auto-detect first available.
	if registry.Default() == nil {
		infos := registry.List()
		if len(infos) > 0 {
			registry.SetFallback(infos[0].Name)
		}
	}

	return registry, nil
}

func loadSkills(engine skills.Engine, dir string, logger *slog.Logger) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Debug("no skills directory found", "dir", dir)
		return
	}

	ctx := context.Background()
	loaded := 0
	skipped := 0
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

		// Check platform requirements
		if !platformAvailable(m.PlatformRequirements) {
			logger.Warn("skipping skill: platform requirement not met",
				"name", m.Name,
				"requires", m.PlatformRequirements,
				"fix", "deploy on a platform with Docker socket access (Droplet, self-hosted)")
			skipped++
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
	if skipped > 0 {
		logger.Info("skills skipped (platform requirements)", "count", skipped)
	}
}

// platformAvailable checks if all platform requirements are satisfied.
func platformAvailable(requirements []string) bool {
	for _, req := range requirements {
		switch req {
		case "docker":
			if _, err := exec.LookPath("docker"); err != nil {
				return false
			}
		}
	}
	return true
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
