package providers

// Route defines a provider routing rule for a channel or tenant.
type Route struct {
	ChannelID string       `json:"channel_id,omitempty" yaml:"channel_id,omitempty"`
	TenantID  string       `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	Provider  string       `json:"provider" yaml:"provider"`
	Model     string       `json:"model,omitempty" yaml:"model,omitempty"`
	Priority  int          `json:"priority" yaml:"priority"`
	Security  *RoutePolicy `json:"security,omitempty" yaml:"security,omitempty"`
}

// RoutePolicy defines per-provider security constraints applied when a route matches.
type RoutePolicy struct {
	MaxTokensPerRequest int    `json:"max_tokens_per_request,omitempty" yaml:"max_tokens_per_request,omitempty"`
	AuditVerbosity      string `json:"audit_verbosity,omitempty" yaml:"audit_verbosity,omitempty"` // "minimal", "standard", "full"
}

// ProviderInfo describes a registered provider's status.
type ProviderInfo struct {
	Name    string `json:"name"`
	Model   string `json:"model"`
	Healthy bool   `json:"healthy"`
}
