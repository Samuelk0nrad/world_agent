package connectors

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Connector interface {
	ID() string
}

type Registry struct {
	mu         sync.RWMutex
	connectors map[string]Connector
}

func NewRegistry() *Registry {
	return &Registry{
		connectors: map[string]Connector{},
	}
}

func (r *Registry) Register(connector Connector) error {
	if connector == nil {
		return fmt.Errorf("connector is required")
	}

	id := normalizeConnectorID(connector.ID())
	if id == "" {
		return fmt.Errorf("connector id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.connectors[id]; exists {
		return fmt.Errorf("connector %q already registered", id)
	}

	r.connectors[id] = connector
	return nil
}

func (r *Registry) Get(id string) (Connector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connector, ok := r.connectors[normalizeConnectorID(id)]
	return connector, ok
}

func (r *Registry) ListIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.connectors))
	for id := range r.connectors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func normalizeConnectorID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
