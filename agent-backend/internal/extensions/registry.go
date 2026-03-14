package extensions

import (
	"fmt"
	"sync"
)

type Extension struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type Registry struct {
	mu         sync.RWMutex
	extensions []Extension
}

func NewDefaultRegistry() *Registry {
	return &Registry{
		extensions: []Extension{
			{
				ID:          "web-search",
				Name:        "Web Search",
				Category:    "knowledge",
				Description: "Searches public web results for research tasks.",
				Enabled:     true,
			},
			{
				ID:          "email",
				Name:        "Email Assistant",
				Category:    "communication",
				Description: "Reads and drafts email actions once configured.",
				Enabled:     false,
			},
			{
				ID:          "mobile-sensors",
				Name:        "Mobile Sensors",
				Category:    "device",
				Description: "Optional screen/audio capabilities controlled by user permissions.",
				Enabled:     false,
			},
		},
	}
}

func (r *Registry) List() []Extension {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Extension, len(r.extensions))
	copy(out, r.extensions)
	return out
}

func (r *Registry) IsEnabled(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ext := range r.extensions {
		if ext.ID == id {
			return ext.Enabled
		}
	}
	return false
}

func (r *Registry) SetEnabled(id string, enabled bool) (Extension, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.extensions {
		if r.extensions[i].ID == id {
			r.extensions[i].Enabled = enabled
			return r.extensions[i], nil
		}
	}
	return Extension{}, fmt.Errorf("extension %q not found", id)
}
