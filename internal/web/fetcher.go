package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/leonardcser/web-mcp/internal/cache"
)

type PageSummary struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Text        string   `json:"text"`
	Links       []string `json:"links"`
}

type Fetcher struct {
	c     *colly.Collector
	cache cache.KV
	ttl   time.Duration
}

func NewFetcher(cacheStore cache.KV, ttl time.Duration) *Fetcher {
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.Async(false),
	)
	c.SetRequestTimeout(20 * time.Second)
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", NextUserAgent())
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
	})
	return &Fetcher{c: c, cache: cacheStore, ttl: ttl}
}

func (f *Fetcher) cacheKey(rawURL string) string { return "web_fetch|" + rawURL }

func (f *Fetcher) Fetch(rawURL string) (*PageSummary, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, errors.New("url must start with http:// or https://")
	}
	if v, err := f.cache.Get(f.cacheKey(rawURL)); err == nil {
		var ps PageSummary
		if json.Unmarshal(v, &ps) == nil {
			return &ps, nil
		}
	}
	var pageHTML []byte
	var finalURL string
	var fetchErr error

	f.c.OnResponse(func(r *colly.Response) {
		finalURL = r.Request.URL.String()
		pageHTML = append([]byte(nil), r.Body...)
	})

	fetchErr = f.c.Visit(rawURL)
	if fetchErr != nil {
		return nil, fetchErr
	}

	if len(pageHTML) == 0 {
		return nil, errors.New("empty response body")
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(pageHTML))
	if err != nil {
		return nil, err
	}

	// Remove non-visible elements
	doc.Find("script, style, noscript, iframe").Each(func(i int, s *goquery.Selection) { s.Remove() })

	title := strings.TrimSpace(doc.Find("head > title").First().Text())
	desc := strings.TrimSpace(doc.Find("meta[name=description]").AttrOr("content", ""))

	// Extract text
	bodyText := strings.TrimSpace(doc.Find("body").Text())
	bodyText = strings.Join(strings.Fields(bodyText), " ")

	// Extract absolute links
	base, _ := url.Parse(finalURL)
	linkSet := make(map[string]struct{})
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href := strings.TrimSpace(s.AttrOr("href", ""))
		if href == "" || strings.HasPrefix(href, "javascript:") {
			return
		}
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		if !u.IsAbs() && base != nil {
			u = base.ResolveReference(u)
		}
		abs := u.String()
		linkSet[abs] = struct{}{}
	})
	links := make([]string, 0, len(linkSet))
	for l := range linkSet {
		links = append(links, l)
	}

	ps := &PageSummary{
		URL:         finalURL,
		Title:       title,
		Description: desc,
		Text:        bodyText,
		Links:       links,
	}
	if b, err := json.Marshal(ps); err == nil {
		_ = f.cache.Put(f.cacheKey(rawURL), b, f.ttl)
	}
	return ps, nil
}
