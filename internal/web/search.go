package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/leonardcser/web-mcp/internal/cache"
)

// extractDDGURL extracts the actual URL from DuckDuckGo's redirect URL format
// Input: //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=...
// Output: https://example.com
func extractDDGURL(ddgURL string) string {
	// Handle protocol-relative URLs
	if strings.HasPrefix(ddgURL, "//duckduckgo.com/l/") {
		ddgURL = "https:" + ddgURL
	}

	u, err := url.Parse(ddgURL)
	if err != nil {
		return ddgURL // Return original if parsing fails
	}

	// Extract the uddg parameter which contains the actual URL
	uddg := u.Query().Get("uddg")
	if uddg == "" {
		return ddgURL // Return original if no uddg parameter
	}

	// URL decode the actual URL
	actualURL, err := url.QueryUnescape(uddg)
	if err != nil {
		return ddgURL // Return original if decoding fails
	}

	return actualURL
}

type SearchResult struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Link        string `json:"link"`
}

type Searcher struct {
	client *http.Client
	cache  cache.KV
	ttl    time.Duration
}

func NewSearcher(cacheStore cache.KV, ttl time.Duration) *Searcher {
	return &Searcher{
		client: &http.Client{Timeout: 15 * time.Second},
		cache:  cacheStore,
		ttl:    ttl,
	}
}

func (s *Searcher) cacheKey(q string) string { return "web_search|" + q }

func (s *Searcher) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("empty query")
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	if v, err := s.cache.Get(s.cacheKey(q)); err == nil {
		var cached []SearchResult
		if json.Unmarshal(v, &cached) == nil {
			if len(cached) > limit {
				return cached[:limit], nil
			}
			return cached, nil
		}
	}
	endpoint := "https://html.duckduckgo.com/html/"
	values := url.Values{"q": {q}, "kl": {"us-en"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", NextUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("duckduckgo status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, limit)
	// Use concrete selectors from the DuckDuckGo HTML endpoint structure.
	doc.Find("div.result.results_links.results_links_deep.web-result").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		a := s.Find("a.result__a").First()
		link := strings.TrimSpace(a.AttrOr("href", ""))
		title := singleLine(a.Text())
		desc := singleLine(s.Find("a.result__snippet").First().Text())
		if title != "" && link != "" {
			// Extract the actual URL from DuckDuckGo's redirect URL
			actualLink := extractDDGURL(link)
			results = append(results, SearchResult{Title: title, Description: desc, Link: actualLink})
		}
		return len(results) < limit
	})

	if len(results) == 0 {
		// Fallback: scan anchor list and nearest snippet up the tree
		doc.Find("a.result__a").EachWithBreak(func(_ int, n *goquery.Selection) bool {
			if len(results) >= limit {
				return false
			}
			title := singleLine(n.Text())
			link := strings.TrimSpace(n.AttrOr("href", ""))
			desc := singleLine(n.Parents().Find("a.result__snippet").First().Text())
			// Extract the actual URL from DuckDuckGo's redirect URL
			actualLink := extractDDGURL(link)
			results = append(results, SearchResult{Title: title, Description: desc, Link: actualLink})
			return true
		})
	}
	if b, err := json.Marshal(results); err == nil {
		_ = s.cache.Put(s.cacheKey(q), b, s.ttl)
	}
	return results, nil
}

// singleLine trims and collapses internal whitespace/newlines to single spaces.
func singleLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	return strings.Join(strings.Fields(s), " ")
}
