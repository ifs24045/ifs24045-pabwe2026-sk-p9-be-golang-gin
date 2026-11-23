package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxSources = 5

// SearchClient mencari informasi terkini ke Search Engine.
type SearchClient interface {
	Search(ctx context.Context, query string) ([]SearchSource, error)
}

type httpSearchClient struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

// NewSearchClient membuat client HTTP untuk Search Engine.
func NewSearchClient(endpoint, apiKey string) SearchClient {
	return &httpSearchClient{
		endpoint: endpoint,
		apiKey:   apiKey,
		http:     &http.Client{Timeout: 20 * time.Second},
	}
}

// Search mengirim GET {endpoint}?q=... lalu mengurai hasilnya menjadi daftar sumber.
func (c *httpSearchClient) Search(ctx context.Context, query string) ([]SearchSource, error) {
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("search engine mengembalikan status %d", resp.StatusCode)
	}

	var raw interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return parseSearchResults(raw), nil
}

// findItems mencari array hasil di JSON: array langsung, atau di bawah kunci
// results / data / items / organic_results (boleh bersarang).
func findItems(raw interface{}) []interface{} {
	switch v := raw.(type) {
	case []interface{}:
		return v
	case map[string]interface{}:
		for _, key := range []string{"results", "data", "items", "organic_results"} {
			if found := findItems(v[key]); found != nil {
				return found
			}
		}
	}
	return nil
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// parseSearchResults mengubah JSON hasil penelusuran menjadi maksimal 5 sumber.
func parseSearchResults(raw interface{}) []SearchSource {
	sources := make([]SearchSource, 0, maxSources)
	for _, item := range findItems(raw) {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		src := SearchSource{
			Title:   firstString(m, "title", "name"),
			URL:     firstString(m, "url", "link", "href"),
			Snippet: firstString(m, "snippet", "content", "description", "body"),
		}
		if src.Title == "" && src.URL == "" {
			continue
		}

		sources = append(sources, src)
		if len(sources) == maxSources {
			break
		}
	}
	return sources
}