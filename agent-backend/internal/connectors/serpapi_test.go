package connectors

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"worldagent/agent-backend/internal/config"
)

type stubWebSearchHTTPClient struct {
	do func(req *http.Request) (*http.Response, error)
}

func (s stubWebSearchHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return s.do(req)
}

func TestNewSerpAPIWebSearchConnectorImplementsConnectorID(t *testing.T) {
	t.Parallel()

	connector, err := NewSerpAPIWebSearchConnector(config.SerpAPIConfig{
		APIKey: "api-key",
		Engine: config.DefaultSerpAPIEngine,
	})
	if err != nil {
		t.Fatalf("new SerpAPI connector: %v", err)
	}
	if connector.ID() != WebSearchConnectorID {
		t.Fatalf("expected connector ID %q, got %q", WebSearchConnectorID, connector.ID())
	}
}

func TestSerpAPIWebSearchConnectorSearchSuccess(t *testing.T) {
	t.Parallel()

	connector, err := NewSerpAPIWebSearchConnector(config.SerpAPIConfig{
		APIKey: "my-key",
		Engine: "google",
	}, WithSerpAPIHTTPClient(stubWebSearchHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", req.Method)
			}
			query := req.URL.Query()
			if query.Get("api_key") != "my-key" {
				t.Fatalf("expected API key in query string, got %q", query.Get("api_key"))
			}
			if query.Get("engine") != "google" {
				t.Fatalf("expected engine google, got %q", query.Get("engine"))
			}
			if query.Get("q") != "golang release notes" {
				t.Fatalf("expected encoded query, got %q", query.Get("q"))
			}
			if query.Get("num") != "2" {
				t.Fatalf("expected num=2, got %q", query.Get("num"))
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"answer_box": {"answer": "Go 1.23 was released recently."},
					"organic_results": [
						{"title": "Go Blog", "link": "https://go.dev/blog", "snippet": "Official release announcement."},
						{"title": "Release Notes", "link": "https://go.dev/doc/devel/release", "snippet": "Detailed release notes."},
						{"title": "Ignored", "link": "https://example.com/ignored", "snippet": "Should not appear due to topK."}
					]
				}`)),
			}, nil
		},
	}))
	if err != nil {
		t.Fatalf("new SerpAPI connector: %v", err)
	}

	result, err := connector.Search(context.Background(), "  golang release notes ", 2)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if result.Query != "golang release notes" {
		t.Fatalf("expected trimmed query, got %q", result.Query)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(result.Sources))
	}
	if result.Sources[0] != "https://go.dev/blog" {
		t.Fatalf("unexpected first source %q", result.Sources[0])
	}
	if !strings.Contains(result.Summary, "Go 1.23 was released recently.") {
		t.Fatalf("expected answer box summary, got %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "Go Blog: Official release announcement.") {
		t.Fatalf("expected organic summary included, got %q", result.Summary)
	}
	if strings.Contains(result.Summary, "Ignored") {
		t.Fatalf("expected summary to respect topK, got %q", result.Summary)
	}
}

func TestSerpAPIWebSearchConnectorSearchValidatesInput(t *testing.T) {
	t.Parallel()

	connector, err := NewSerpAPIWebSearchConnector(config.SerpAPIConfig{
		APIKey: "my-key",
		Engine: config.DefaultSerpAPIEngine,
	})
	if err != nil {
		t.Fatalf("new SerpAPI connector: %v", err)
	}

	_, err = connector.Search(context.Background(), "  ", 3)
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query validation error, got %v", err)
	}

	_, err = connector.Search(context.Background(), "golang", 0)
	if err == nil || !strings.Contains(err.Error(), "topK must be greater than 0") {
		t.Fatalf("expected topK validation error, got %v", err)
	}
}

func TestSerpAPIWebSearchConnectorSearchReturnsTransportError(t *testing.T) {
	t.Parallel()

	connector, err := NewSerpAPIWebSearchConnector(config.SerpAPIConfig{
		APIKey: "my-key",
		Engine: config.DefaultSerpAPIEngine,
	}, WithSerpAPIHTTPClient(stubWebSearchHTTPClient{
		do: func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial timeout")
		},
	}))
	if err != nil {
		t.Fatalf("new SerpAPI connector: %v", err)
	}

	_, err = connector.Search(context.Background(), "golang", 2)
	if err == nil || !strings.Contains(err.Error(), "call SerpAPI: dial timeout") {
		t.Fatalf("expected wrapped transport error, got %v", err)
	}
}

func TestSerpAPIWebSearchConnectorSearchReturnsAPIError(t *testing.T) {
	t.Parallel()

	connector, err := NewSerpAPIWebSearchConnector(config.SerpAPIConfig{
		APIKey: "my-key",
		Engine: config.DefaultSerpAPIEngine,
	}, WithSerpAPIHTTPClient(stubWebSearchHTTPClient{
		do: func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`rate limit exceeded`)),
			}, nil
		},
	}))
	if err != nil {
		t.Fatalf("new SerpAPI connector: %v", err)
	}

	_, err = connector.Search(context.Background(), "golang", 1)
	if err == nil || !strings.Contains(err.Error(), "status 429") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestSerpAPIWebSearchConnectorSearchReturnsDecodeError(t *testing.T) {
	t.Parallel()

	connector, err := NewSerpAPIWebSearchConnector(config.SerpAPIConfig{
		APIKey: "my-key",
		Engine: config.DefaultSerpAPIEngine,
	}, WithSerpAPIHTTPClient(stubWebSearchHTTPClient{
		do: func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{`)),
			}, nil
		},
	}))
	if err != nil {
		t.Fatalf("new SerpAPI connector: %v", err)
	}

	_, err = connector.Search(context.Background(), "golang", 1)
	if err == nil || !strings.Contains(err.Error(), "decode SerpAPI response") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestSerpAPIWebSearchConnectorSearchReturnsUsableResultsError(t *testing.T) {
	t.Parallel()

	connector, err := NewSerpAPIWebSearchConnector(config.SerpAPIConfig{
		APIKey: "my-key",
		Engine: config.DefaultSerpAPIEngine,
	}, WithSerpAPIHTTPClient(stubWebSearchHTTPClient{
		do: func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"organic_results": []}`)),
			}, nil
		},
	}))
	if err != nil {
		t.Fatalf("new SerpAPI connector: %v", err)
	}

	_, err = connector.Search(context.Background(), "golang", 1)
	if err == nil || !strings.Contains(err.Error(), "does not contain usable search results") {
		t.Fatalf("expected usable results error, got %v", err)
	}
}
