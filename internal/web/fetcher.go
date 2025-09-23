package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/leonardcser/web-mcp/internal/cache"
)

const (
	RequestTimeout  = 20 * time.Second
	MaxResponseSize = 1 * 1024 * 1024 // 1MB
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
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       1 * time.Second,
	})
	c.SetRequestTimeout(RequestTimeout)
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", NextUserAgent())
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
	})
	return &Fetcher{c: c, cache: cacheStore, ttl: ttl}
}

func (f *Fetcher) cacheKey(rawURL string) string { return "web_fetch|" + rawURL }

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*PageSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
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

	var contentType string

	originalCtx := f.c.Context
	f.c.Context = ctx
	defer func() { f.c.Context = originalCtx }()

	f.c.OnResponse(func(r *colly.Response) {
		if ctx.Err() != nil {
			return
		}
		finalURL = r.Request.URL.String()
		pageHTML = append([]byte(nil), r.Body...)
		contentType = r.Headers.Get("Content-Type")
	})

	fetchErr = f.c.Visit(rawURL)
	if fetchErr != nil {
		return nil, fetchErr
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if len(pageHTML) == 0 {
		return nil, errors.New("empty response body")
	}

	if len(pageHTML) > MaxResponseSize {
		pageHTML = pageHTML[:MaxResponseSize]
		pageHTML = append(pageHTML, []byte("... [response trimmed due to size]")...)
	}

	lowerCT := strings.ToLower(contentType)
	isHTML := strings.Contains(lowerCT, "text/html")
	isText := strings.HasPrefix(lowerCT, "text/")

	if !isText {
		return nil, errors.New("unsupported content type: binary files like images or PDFs are not supported")
	}

	var title, desc, bodyText string
	var links []string

	if isHTML {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(pageHTML))
		if err != nil {
			return nil, err
		}

		// Remove non-visible elements
		doc.Find("script, style, noscript, iframe, object, embed, img, video, picture, svg, canvas, audio, source, track, map, area, form, label, input, button, select, textarea, progress, ins, applet").Remove()

		title = strings.TrimSpace(doc.Find("head > title").First().Text())
		desc = strings.TrimSpace(doc.Find("meta[name=description]").AttrOr("content", ""))

		plainText := strings.TrimSpace(doc.Find("body").Text())
		plainText = strings.Join(strings.Fields(plainText), " ")

		// Extract absolute links with filtering and deduplication
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

		// Filter, dedupe canonical, exclude unwanted schemes, remove fragments, limit to 50
		canonicalSet := make(map[string]struct{})
		for abs := range linkSet {
			u, err := url.Parse(abs)
			if err != nil {
				continue
			}
			// Exclude unwanted schemes
			if u.Scheme == "javascript" || u.Scheme == "mailto" || u.Scheme == "tel" || u.Scheme == "" {
				continue
			}
			// Remove fragment
			u.Fragment = ""
			canon := u.String()
			canonicalSet[canon] = struct{}{}
		}

		links = make([]string, 0, len(canonicalSet))
		for canon := range canonicalSet {
			links = append(links, canon)
			if len(links) >= 50 {
				break
			}
		}
		sort.Strings(links)

		// Remove &lt;a&gt; elements after extracting links
		doc.Find("a").Remove()

		// Remove header and footer
		doc.Find("header, footer, aside").Remove()

		// Convert to Markdown
		htmlStr, err := doc.Html()
		if err != nil {
			return nil, err
		}

		markdown, err := htmltomarkdown.ConvertString(string(htmlStr))
		if err != nil {
			bodyText = plainText
		} else {
			bodyText = markdown
		}
	} else {
		bodyText = string(pageHTML)
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
