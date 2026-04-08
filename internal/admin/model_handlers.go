package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ModelInfo describes an available model for a provider.
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	Current     bool   `json:"current"`
}

// Curated model lists for providers that don't have a listing API.
var curatedModels = map[string][]ModelInfo{
	"anthropic": {
		{ID: "claude-sonnet-4-20250514", DisplayName: "Claude Sonnet 4"},
		{ID: "claude-opus-4-20250514", DisplayName: "Claude Opus 4"},
		{ID: "claude-haiku-3-5-20241022", DisplayName: "Claude 3.5 Haiku"},
	},
	"openai": {
		{ID: "gpt-4o", DisplayName: "GPT-4o"},
		{ID: "gpt-4o-mini", DisplayName: "GPT-4o Mini"},
		{ID: "o3-mini", DisplayName: "o3 Mini"},
		{ID: "o1", DisplayName: "o1"},
	},
}

// handleListModels returns available models for a provider.
// For Gradient, queries the live API. For others, returns curated lists.
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	currentModel := ""
	if p, ok := s.registry.Get(name); ok {
		currentModel = p.ModelID()
	}

	if name == "gradient" {
		models, err := s.fetchGradientModels(r.Context(), currentModel)
		if err != nil {
			s.logger.Warn("failed to fetch gradient models", "err", err)
			// Fall back to just showing the current model
			if currentModel != "" {
				s.jsonOK(w, []ModelInfo{{ID: currentModel, Current: true}})
			} else {
				s.jsonOK(w, []ModelInfo{})
			}
			return
		}
		s.jsonOK(w, models)
		return
	}

	if curated, ok := curatedModels[name]; ok {
		result := make([]ModelInfo, len(curated))
		copy(result, curated)
		// Mark current model
		found := false
		for i := range result {
			if result[i].ID == currentModel {
				result[i].Current = true
				found = true
			}
		}
		// If current model isn't in curated list, add it
		if !found && currentModel != "" {
			result = append([]ModelInfo{{ID: currentModel, Current: true}}, result...)
		}
		s.jsonOK(w, result)
		return
	}

	s.jsonError(w, fmt.Sprintf("unknown provider %q", name), http.StatusNotFound)
}

// fetchGradientModels calls the DO Gradient /v1/models endpoint.
func (s *Server) fetchGradientModels(ctx context.Context, currentModel string) ([]ModelInfo, error) {
	// Get API key from registry or secrets store
	apiKey := ""
	if s.secrets != nil {
		key, err := s.secrets.Get(ctx, "gradient")
		if err == nil && key != "" {
			apiKey = key
		}
	}
	if apiKey == "" {
		return nil, fmt.Errorf("no gradient API key available")
	}

	baseURL := "https://inference.do-ai.run/v1"
	if s.config.Providers.Gradient != nil && s.config.Providers.Gradient.BaseURL != "" {
		baseURL = s.config.Providers.Gradient.BaseURL
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gradient API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, err
	}

	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(response.Data))
	for _, m := range response.Data {
		models = append(models, ModelInfo{
			ID:      m.ID,
			Current: m.ID == currentModel,
		})
	}
	return models, nil
}

type setModelRequest struct {
	Model string `json:"model"`
}

// handleSetModel changes the active model for a provider (runtime only).
func (s *Server) handleSetModel(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req setModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		s.jsonError(w, "model is required", http.StatusBadRequest)
		return
	}

	// Get current API key
	existing, ok := s.registry.Get(name)
	if !ok {
		s.jsonError(w, fmt.Sprintf("provider %q is not configured", name), http.StatusNotFound)
		return
	}

	// If already using this model, no-op
	if existing.ModelID() == req.Model {
		s.jsonOK(w, map[string]string{"status": "unchanged", "model": req.Model})
		return
	}

	// Get the API key to re-create the provider
	apiKey := ""
	if s.secrets != nil {
		key, err := s.secrets.Get(r.Context(), name)
		if err == nil && key != "" {
			apiKey = key
		}
	}
	// Fall back to env var
	if apiKey == "" {
		envVars := map[string]string{
			"anthropic": "ANTHROPIC_API_KEY",
			"openai":    "OPENAI_API_KEY",
			"gradient":  "MODEL_ACCESS_KEY",
		}
		if envVar, ok := envVars[name]; ok {
			apiKey = os.Getenv(envVar)
		}
	}
	if apiKey == "" {
		s.jsonError(w, "cannot retrieve API key to switch model", http.StatusInternalServerError)
		return
	}

	// Re-create provider with new model
	p, err := createProvider(name, apiKey, req.Model, "", s.config)
	if err != nil {
		s.logger.Warn("failed to create provider with new model", "provider", name, "model", req.Model, "err", err)
		s.jsonError(w, "failed to switch model", http.StatusBadRequest)
		return
	}

	s.registry.Register(name, p)
	s.logger.Info("provider model switched", "provider", name, "model", req.Model)

	s.jsonOK(w, map[string]string{"status": "updated", "model": req.Model})
}
