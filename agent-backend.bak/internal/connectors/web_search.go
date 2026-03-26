package connectors

import (
	"context"
	"fmt"
	"strings"
)

const WebSearchConnectorID = "web-search"

type SearchResult struct {
	Query   string
	Summary string
	Sources []string
}

type WebSearchConnector interface {
	Connector
	Search(ctx context.Context, query string, topK int) (SearchResult, error)
}

type unavailableWebSearchConnector struct {
	err error
}

func NewUnavailableWebSearchConnector(err error) WebSearchConnector {
	return &unavailableWebSearchConnector{err: err}
}

func (c *unavailableWebSearchConnector) ID() string {
	return WebSearchConnectorID
}

func (c *unavailableWebSearchConnector) Search(_ context.Context, query string, topK int) (SearchResult, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return SearchResult{}, fmt.Errorf("query is required")
	}
	if topK <= 0 {
		return SearchResult{}, fmt.Errorf("topK must be greater than 0")
	}
	if c.err != nil {
		return SearchResult{Query: trimmed}, c.err
	}

	return SearchResult{Query: trimmed}, fmt.Errorf("web-search connector is unavailable")
}

func GetWebSearchConnector(registry *Registry) (WebSearchConnector, error) {
	if registry == nil {
		return nil, fmt.Errorf("connector registry is not configured")
	}

	connector, ok := registry.Get(WebSearchConnectorID)
	if !ok {
		return nil, fmt.Errorf("connector %q is not registered", WebSearchConnectorID)
	}

	webSearch, ok := connector.(WebSearchConnector)
	if !ok {
		return nil, fmt.Errorf("connector %q does not implement web search", WebSearchConnectorID)
	}

	return webSearch, nil
}
