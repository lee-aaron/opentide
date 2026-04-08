package admin

import (
	"context"
	"encoding/json"
	"net/http"
)

type toggleSkillRequest struct {
	Enabled bool `json:"enabled"`
}

// handleToggleSkill enables or disables a skill by tool name.
// Disabled skills are retained in memory for re-enabling without restart.
// The disabled state is persisted via the secrets store under key "_disabled_skills".
func (s *Server) handleToggleSkill(w http.ResponseWriter, r *http.Request) {
	if s.skills == nil {
		s.jsonError(w, "skill engine not configured", http.StatusInternalServerError)
		return
	}

	toolName := r.PathValue("tool_name")
	if toolName == "" {
		s.jsonError(w, "tool_name is required", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req toggleSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var err error
	if req.Enabled {
		err = s.skills.EnableSkill(ctx, toolName)
	} else {
		err = s.skills.DisableSkill(ctx, toolName)
	}
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	// Persist disabled state
	s.persistDisabledSkills(ctx)

	action := "disabled"
	if req.Enabled {
		action = "enabled"
	}
	s.logger.Info("skill toggled", "tool", toolName, "action", action)

	s.jsonOK(w, map[string]any{
		"status":    action,
		"tool_name": toolName,
	})
}

// persistDisabledSkills saves the list of disabled skill tool names to the secrets store.
func (s *Server) persistDisabledSkills(ctx context.Context) {
	if s.secrets == nil || s.skills == nil {
		return
	}

	infos, err := s.skills.ListSkills(ctx)
	if err != nil {
		return
	}

	var disabled []string
	for _, info := range infos {
		if !info.Enabled {
			disabled = append(disabled, info.ToolName)
		}
	}

	data, _ := json.Marshal(disabled)
	_, _ = s.secrets.Put(ctx, "_disabled_skills", string(data))
}

// LoadDisabledSkills reads the persisted disabled skills list and returns tool names.
func LoadDisabledSkills(secretStore interface {
	Get(ctx context.Context, provider string) (string, error)
}, ctx context.Context) []string {
	if secretStore == nil {
		return nil
	}

	data, err := secretStore.Get(ctx, "_disabled_skills")
	if err != nil || data == "" {
		return nil
	}

	var names []string
	if err := json.Unmarshal([]byte(data), &names); err != nil {
		return nil
	}
	return names
}
