// Package gradient implements the DigitalOcean Gradient AI Platform provider.
// It reuses the OpenAI adapter since Gradient exposes an OpenAI-compatible API.
package gradient

import (
	openaiProvider "github.com/opentide/opentide/internal/providers/openai"
	oerr "github.com/opentide/opentide/pkg/errors"
)

const (
	defaultBaseURL = "https://inference.do-ai.run/v1"
	defaultModel   = "meta-llama/Llama-3.3-70B-Instruct"
)

// New creates a DO Gradient provider.
func New(apiKey, model, baseURL string) (*openaiProvider.Provider, error) {
	if apiKey == "" {
		return nil, oerr.New(oerr.CodeProviderAuth, "DO Gradient API key is empty").
			WithFix("Set the DO_GRADIENT_API_KEY environment variable").
			WithDocs("https://docs.digitalocean.com/products/gradient/")
	}
	if model == "" {
		model = defaultModel
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return openaiProvider.NewWithBaseURL(apiKey, model, baseURL, "gradient")
}
