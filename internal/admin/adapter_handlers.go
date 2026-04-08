package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/opentide/opentide/internal/security/secrets"
)

// knownAdapters maps adapter secret names to their corresponding environment variables.
var knownAdapters = map[string]string{
	"discord":   "DISCORD_TOKEN",
	"slack_bot": "SLACK_BOT_TOKEN",
	"slack_app": "SLACK_APP_TOKEN",
}

// handleListAdapterSecrets returns configuration status for all known adapters.
func (s *Server) handleListAdapterSecrets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var result []secrets.SecretMeta

	stored := make(map[string]secrets.SecretMeta)
	if s.secrets != nil {
		list, _ := s.secrets.List(ctx)
		for _, m := range list {
			stored[m.Provider] = m
		}
	}

	for adapter, envVar := range knownAdapters {
		envVal := os.Getenv(envVar)
		if envVal != "" {
			result = append(result, secrets.SecretMeta{
				Provider:   adapter,
				Last4:      maskLast4(envVal),
				Source:     "env",
				Configured: true,
			})
			continue
		}

		if m, ok := stored[adapter]; ok {
			result = append(result, m)
			continue
		}

		result = append(result, secrets.SecretMeta{
			Provider:   adapter,
			Configured: false,
		})
	}

	s.jsonOK(w, result)
}

type setAdapterSecretRequest struct {
	Adapter string `json:"adapter"`
	Token   string `json:"token"`
}

// handleSetAdapterSecret stores an encrypted adapter token.
// Unlike provider secrets, adapter tokens cannot be hot-reloaded — a restart is required.
func (s *Server) handleSetAdapterSecret(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		s.jsonError(w, "secrets store not configured", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req setAdapterSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Adapter = strings.ToLower(strings.TrimSpace(req.Adapter))
	envVar, ok := knownAdapters[req.Adapter]
	if !ok {
		s.jsonError(w, fmt.Sprintf("unknown adapter %q (valid: discord, slack_bot, slack_app)", req.Adapter), http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		s.jsonError(w, "token is required", http.StatusBadRequest)
		return
	}

	if os.Getenv(envVar) != "" {
		s.jsonError(w, fmt.Sprintf("adapter %q is configured via %s environment variable; remove the env var first", req.Adapter, envVar), http.StatusConflict)
		return
	}

	meta, err := s.secrets.Put(r.Context(), req.Adapter, req.Token)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to store secret: %v", err), http.StatusInternalServerError)
		return
	}

	s.logger.Info("adapter token stored (restart required to take effect)", "adapter", req.Adapter)

	w.WriteHeader(http.StatusCreated)
	s.jsonOK(w, map[string]any{
		"meta":             meta,
		"restart_required": true,
	})
}

// handleDeleteAdapterSecret removes a stored adapter token.
func (s *Server) handleDeleteAdapterSecret(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		s.jsonError(w, "secrets store not configured", http.StatusInternalServerError)
		return
	}

	adapter := r.PathValue("adapter")
	envVar, ok := knownAdapters[adapter]
	if !ok {
		s.jsonError(w, fmt.Sprintf("unknown adapter %q", adapter), http.StatusBadRequest)
		return
	}

	if os.Getenv(envVar) != "" {
		s.jsonError(w, fmt.Sprintf("adapter %q is configured via %s environment variable; cannot delete", adapter, envVar), http.StatusConflict)
		return
	}

	if err := s.secrets.Delete(r.Context(), adapter); err != nil {
		s.jsonError(w, "secret not found", http.StatusNotFound)
		return
	}

	s.logger.Info("adapter token deleted (restart required to take effect)", "adapter", adapter)

	s.jsonOK(w, map[string]any{
		"status":           "deleted",
		"restart_required": true,
	})
}
