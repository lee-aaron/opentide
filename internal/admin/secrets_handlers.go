package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/opentide/opentide/internal/config"
	"github.com/opentide/opentide/internal/providers"
	anthropicProvider "github.com/opentide/opentide/internal/providers/anthropic"
	"github.com/opentide/opentide/internal/providers/gradient"
	openaiProvider "github.com/opentide/opentide/internal/providers/openai"
	"github.com/opentide/opentide/internal/security/secrets"
)

// knownProviders lists valid provider names and the env var that configures each.
var knownProviders = map[string]string{
	"anthropic": "ANTHROPIC_API_KEY",
	"openai":    "OPENAI_API_KEY",
	"gradient":  "MODEL_ACCESS_KEY",
}

// handleListSecrets returns configuration status for all known providers.
// Env-var-configured providers show source="env". Store-configured show source="store".
// Unconfigured providers show configured=false.
func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var result []secrets.SecretMeta

	// Get stored secrets
	stored := make(map[string]secrets.SecretMeta)
	if s.secrets != nil {
		list, _ := s.secrets.List(ctx)
		for _, m := range list {
			stored[m.Provider] = m
		}
	}

	for provider, envVar := range knownProviders {
		envVal := os.Getenv(envVar)
		if envVal != "" {
			// Env var takes precedence
			result = append(result, secrets.SecretMeta{
				Provider:   provider,
				Last4:      maskLast4(envVal),
				Source:     "env",
				Configured: true,
			})
			continue
		}

		if m, ok := stored[provider]; ok {
			result = append(result, m)
			continue
		}

		// Not configured
		result = append(result, secrets.SecretMeta{
			Provider:   provider,
			Configured: false,
		})
	}

	s.jsonOK(w, result)
}

type setSecretRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
}

// handleSetSecret stores an encrypted API key and hot-reloads the provider.
func (s *Server) handleSetSecret(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		s.jsonError(w, "secrets store not configured", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req setSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	if _, ok := knownProviders[req.Provider]; !ok {
		s.jsonError(w, fmt.Sprintf("unknown provider %q (valid: anthropic, openai, gradient)", req.Provider), http.StatusBadRequest)
		return
	}
	if req.APIKey == "" {
		s.jsonError(w, "api_key is required", http.StatusBadRequest)
		return
	}

	// Check if env var is already set (env takes precedence)
	envVar := knownProviders[req.Provider]
	if os.Getenv(envVar) != "" {
		s.jsonError(w, fmt.Sprintf("provider %q is configured via %s environment variable; remove the env var first to use UI-managed keys", req.Provider, envVar), http.StatusConflict)
		return
	}

	// Try to create the provider first to validate the key format.
	// Never expose provider SDK errors to users (may reveal internals).
	p, err := createProvider(req.Provider, req.APIKey, req.Model, req.BaseURL, s.config)
	if err != nil {
		s.logger.Warn("provider creation failed for stored secret", "provider", req.Provider, "err", err)
		s.jsonError(w, "invalid API key or configuration", http.StatusBadRequest)
		return
	}

	// Store encrypted
	meta, err := s.secrets.Put(r.Context(), req.Provider, req.APIKey)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to store secret: %v", err), http.StatusInternalServerError)
		return
	}

	// Hot-reload: register the new provider
	if s.registry != nil {
		s.registry.Register(req.Provider, p)
		// Auto-set fallback if this is the first provider
		if s.registry.Default() == nil {
			s.registry.SetFallback(req.Provider)
		}
		s.logger.Info("provider hot-loaded from stored secret", "provider", req.Provider, "model", p.ModelID())
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonOK(w, meta)
}

// handleDeleteSecret removes a stored API key and unregisters the provider.
func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		s.jsonError(w, "secrets store not configured", http.StatusInternalServerError)
		return
	}

	provider := r.PathValue("provider")
	if _, ok := knownProviders[provider]; !ok {
		s.jsonError(w, fmt.Sprintf("unknown provider %q", provider), http.StatusBadRequest)
		return
	}

	// Don't delete if env var is the source
	envVar := knownProviders[provider]
	if os.Getenv(envVar) != "" {
		s.jsonError(w, fmt.Sprintf("provider %q is configured via %s environment variable; cannot delete", provider, envVar), http.StatusConflict)
		return
	}

	if err := s.secrets.Delete(r.Context(), provider); err != nil {
		s.jsonError(w, "secret not found", http.StatusNotFound)
		return
	}

	// Unregister the provider
	if s.registry != nil {
		s.registry.Unregister(provider)
		s.logger.Info("provider unregistered after secret deletion", "provider", provider)
	}

	s.jsonOK(w, map[string]string{"status": "deleted"})
}

// createProvider instantiates a provider from an API key, validating the config.
func createProvider(name, apiKey, model, baseURL string, cfg *config.Config) (providers.Provider, error) {
	switch name {
	case "anthropic":
		if model == "" {
			if cfg.Providers.Anthropic != nil && cfg.Providers.Anthropic.Model != "" {
				model = cfg.Providers.Anthropic.Model
			} else {
				model = "claude-sonnet-4-20250514"
			}
		}
		return anthropicProvider.New(apiKey, model)
	case "openai":
		if model == "" {
			if cfg.Providers.OpenAI != nil && cfg.Providers.OpenAI.Model != "" {
				model = cfg.Providers.OpenAI.Model
			} else {
				model = "gpt-4o"
			}
		}
		return openaiProvider.New(apiKey, model)
	case "gradient":
		if model == "" {
			if cfg.Providers.Gradient != nil && cfg.Providers.Gradient.Model != "" {
				model = cfg.Providers.Gradient.Model
			} else {
				model = "meta-llama/Llama-3.3-70B-Instruct"
			}
		}
		if baseURL == "" {
			if cfg.Providers.Gradient != nil && cfg.Providers.Gradient.BaseURL != "" {
				baseURL = cfg.Providers.Gradient.BaseURL
			}
		}
		return gradient.New(apiKey, model, baseURL)
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

func maskLast4(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[len(s)-4:]
}
