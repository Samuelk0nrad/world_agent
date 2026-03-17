package ai

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: map[string]Provider{},
	}
}

func (r *ProviderRegistry) Register(provider Provider) error {
	if provider == nil {
		return fmt.Errorf("provider is required")
	}

	providerID := normalizeProviderID(provider.ID())
	if providerID == "" {
		return fmt.Errorf("provider id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[providerID]; exists {
		return fmt.Errorf("provider %q already registered", providerID)
	}

	r.providers[providerID] = provider
	return nil
}

func (r *ProviderRegistry) Provider(providerID string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[normalizeProviderID(providerID)]
	return provider, ok
}

func (r *ProviderRegistry) NewAssistantModule(providerID string, model ModelDescriptor) (AssistantModule, error) {
	if err := ValidateModelDescriptor(model); err != nil {
		return nil, err
	}

	// Provider selection can come directly from the requested provider ID or from
	// model metadata, allowing callers to stay provider-agnostic when desired.
	normalizedProviderID := normalizeProviderID(providerID)
	if normalizedProviderID == "" {
		normalizedProviderID = normalizeProviderID(model.ProviderID())
	}
	if normalizedProviderID == "" {
		return nil, fmt.Errorf("provider id is required")
	}
	modelProviderID := normalizeProviderID(model.ProviderID())
	if modelProviderID != "" && modelProviderID != normalizedProviderID {
		return nil, fmt.Errorf("model provider %q does not match requested provider %q", modelProviderID, normalizedProviderID)
	}

	provider, ok := r.Provider(normalizedProviderID)
	if !ok {
		return nil, fmt.Errorf("provider %q is not registered", normalizedProviderID)
	}
	return provider.NewAssistantModule(model)
}

func (r *ProviderRegistry) ListProviderIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.providers))
	for providerID := range r.providers {
		ids = append(ids, providerID)
	}
	sort.Strings(ids)
	return ids
}

var defaultProviderRegistry = NewProviderRegistry()

func RegisterProvider(provider Provider) error {
	return defaultProviderRegistry.Register(provider)
}

func RegisterProviderFactory(providerID string, factory AssistantModuleFactory) error {
	return defaultProviderRegistry.Register(NewStaticProvider(providerID, factory))
}

func ProviderByID(providerID string) (Provider, bool) {
	return defaultProviderRegistry.Provider(providerID)
}

func RegisteredProviderIDs() []string {
	return defaultProviderRegistry.ListProviderIDs()
}

func NewAssistantModule(providerID string, model ModelDescriptor) (AssistantModule, error) {
	return defaultProviderRegistry.NewAssistantModule(providerID, model)
}

func normalizeProviderID(providerID string) string {
	return strings.ToLower(strings.TrimSpace(providerID))
}
