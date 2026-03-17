package gemini

import "agent-backend/pkg/gai/ai"

type Provider struct {
	apiKey string
}

func New(apiKey string) *Provider {
	return &Provider{apiKey: apiKey}
}

func (p *Provider) Name() string {
	return "gemini"
}

func (p *Provider) Model(name string) ai.Model {
	return &Model{
		name:   name,
		client: p,
	}
}
