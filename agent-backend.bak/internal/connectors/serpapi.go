package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"worldagent/agent-backend/internal/config"
)

const defaultSerpAPIBaseURL = "https://serpapi.com"

type webSearchHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type SerpAPIWebSearchConnector struct {
	apiKey     string
	engine     string
	baseURL    string
	httpClient webSearchHTTPClient
}

type SerpAPIOption func(*SerpAPIWebSearchConnector)

func WithSerpAPIBaseURL(baseURL string) SerpAPIOption {
	return func(connector *SerpAPIWebSearchConnector) {
		trimmed := strings.TrimSpace(baseURL)
		if trimmed != "" {
			connector.baseURL = strings.TrimRight(trimmed, "/")
		}
	}
}

func WithSerpAPIHTTPClient(httpClient webSearchHTTPClient) SerpAPIOption {
	return func(connector *SerpAPIWebSearchConnector) {
		if httpClient != nil {
			connector.httpClient = httpClient
		}
	}
}

func NewSerpAPIWebSearchConnector(cfg config.SerpAPIConfig, options ...SerpAPIOption) (*SerpAPIWebSearchConnector, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	connector := &SerpAPIWebSearchConnector{
		apiKey:     cfg.APIKey,
		engine:     cfg.Engine,
		baseURL:    defaultSerpAPIBaseURL,
		httpClient: http.DefaultClient,
	}
	for _, option := range options {
		option(connector)
	}
	return connector, nil
}

func NewSerpAPIWebSearchConnectorFromEnv(options ...SerpAPIOption) (*SerpAPIWebSearchConnector, error) {
	cfg := config.LoadSerpAPIConfig()
	return NewSerpAPIWebSearchConnector(cfg, options...)
}

func (c *SerpAPIWebSearchConnector) ID() string {
	return WebSearchConnectorID
}

func (c *SerpAPIWebSearchConnector) Search(ctx context.Context, query string, topK int) (SearchResult, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return SearchResult{}, fmt.Errorf("query is required")
	}
	if topK <= 0 {
		return SearchResult{}, fmt.Errorf("topK must be greater than 0")
	}

	params := url.Values{}
	params.Set("engine", c.engine)
	params.Set("q", trimmedQuery)
	params.Set("api_key", c.apiKey)
	params.Set("num", strconv.Itoa(topK))

	endpoint := fmt.Sprintf("%s/search.json?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return SearchResult{}, fmt.Errorf("create SerpAPI request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return SearchResult{}, fmt.Errorf("call SerpAPI: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return SearchResult{}, fmt.Errorf("read SerpAPI response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return SearchResult{}, fmt.Errorf("SerpAPI request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	var decoded struct {
		AnswerBox struct {
			Answer  string `json:"answer"`
			Snippet string `json:"snippet"`
		} `json:"answer_box"`
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	}
	if err := json.Unmarshal(rawBody, &decoded); err != nil {
		return SearchResult{}, fmt.Errorf("decode SerpAPI response: %w", err)
	}

	summaryParts := make([]string, 0, topK+1)
	if answer := strings.TrimSpace(decoded.AnswerBox.Answer); answer != "" {
		summaryParts = append(summaryParts, answer)
	} else if snippet := strings.TrimSpace(decoded.AnswerBox.Snippet); snippet != "" {
		summaryParts = append(summaryParts, snippet)
	}

	sources := make([]string, 0, topK)
	selectedResults := 0
	for _, result := range decoded.OrganicResults {
		if selectedResults >= topK {
			break
		}

		title := strings.TrimSpace(result.Title)
		snippet := strings.TrimSpace(result.Snippet)
		link := strings.TrimSpace(result.Link)
		if title == "" && snippet == "" && link == "" {
			continue
		}
		selectedResults++

		switch {
		case title != "" && snippet != "":
			summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", title, snippet))
		case snippet != "":
			summaryParts = append(summaryParts, snippet)
		case title != "":
			summaryParts = append(summaryParts, title)
		}

		if link != "" {
			sources = append(sources, link)
		}
	}

	if len(summaryParts) == 0 {
		return SearchResult{}, fmt.Errorf("SerpAPI response does not contain usable search results")
	}

	return SearchResult{
		Query:   trimmedQuery,
		Summary: strings.Join(summaryParts, " "),
		Sources: sources,
	}, nil
}

var _ WebSearchConnector = (*SerpAPIWebSearchConnector)(nil)
