package scraper

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/hk28/prman/internal/cache"
	"github.com/hk28/prman/internal/config"
)

type Scraper struct {
	client      *http.Client
	rateLimit   time.Duration
	lastRequest time.Time
	userAgent   string
}

func New() *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimit: 2 * time.Second,
		userAgent: "prman/1.0 (personal media manager)",
	}
}

func (s *Scraper) FetchIssue(cfg config.SeriesConfig, number int) (cache.ScrapedIssue, error) {
	// Rate limit
	if wait := s.rateLimit - time.Since(s.lastRequest); wait > 0 {
		time.Sleep(wait)
	}
	s.lastRequest = time.Now()

	url := fmt.Sprintf(cfg.URL, number)
	log.Printf("scraping %s #%d ...", cfg.Name, number)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return cache.ScrapedIssue{}, err
	}
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return cache.ScrapedIssue{}, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return cache.ScrapedIssue{}, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	finalURL := resp.Request.URL.String()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return cache.ScrapedIssue{}, fmt.Errorf("parsing HTML: %w", err)
	}

	// Extract fields defined in series config
	fields := make(map[string]string)
	for _, vm := range cfg.Values {
		fields[vm.Name] = extractField(doc, vm.Alias)
	}

	issue := cache.ScrapedIssue{
		Number:      number,
		Title:       fields["Title"],
		Description: fields["Description"],
		Author:      fields["Author"],
		ReleaseDate: normalizeDate(fields["ReleaseDate"]),
		SubSeries:   fields["SubSeries"],
		CoverURL:    extractCoverURL(doc),
		SourceURL:   finalURL,
		CachedAt:    time.Now(),
	}
	log.Printf("scraped %s #%d: title=%q date=%s", cfg.Name, number, issue.Title, issue.ReleaseDate)

	// Put any non-standard fields into Extra
	standardNames := map[string]bool{"Title": true, "Description": true, "Author": true, "ReleaseDate": true, "SubSeries": true}
	for k, v := range fields {
		if !standardNames[k] && v != "" {
			if issue.Extra == nil {
				issue.Extra = make(map[string]string)
			}
			issue.Extra[k] = v
		}
	}

	return issue, nil
}

// extractField walks the HTML tree looking for a <td> whose text equals alias,
// then returns the text of the next sibling <td> in the same <tr>.
// normalizeText replaces non-breaking spaces (&#160; / \u00A0) with regular spaces.
func normalizeText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\u00A0", " "))
}

func extractField(doc *html.Node, alias string) string {
	alias = normalizeText(alias)
	var result string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			// Collect <th> and <td> children (Perrypedia uses <th> for some labels)
			var cells []*html.Node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cells = append(cells, c)
				}
			}
			for i, cell := range cells {
				if normalizeText(textContent(cell)) == alias && i+1 < len(cells) {
					result = normalizeText(textContent(cells[i+1]))
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return result
}

// textContent recursively collects text from a node tree.
func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}

// imgSrc returns the image URL from src or data-src attributes (handles lazy-loading).
func imgSrc(n *html.Node) string {
	src := ""
	for _, a := range n.Attr {
		if a.Key == "src" {
			src = a.Val
		} else if a.Key == "data-src" && src == "" {
			src = a.Val
		}
	}
	return src
}

// extractCoverURL finds the first cover image in the Perrypedia infobox.
// Perrypedia uses protocol-relative URLs like //www.perrypedia.de/mediawiki/images/...
func extractCoverURL(doc *html.Node) string {
	var result string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			src := imgSrc(n)
			if strings.Contains(src, "/mediawiki/images/") {
				if strings.HasPrefix(src, "//") {
					src = "https:" + src
				} else if strings.HasPrefix(src, "/") {
					src = "https://www.perrypedia.de" + src
				}
				result = src
				log.Printf("cover URL found: %s", src)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return result
}

// normalizeDate tries to parse German date formats and return "YYYY-MM-DD".
// Perrypedia typically returns dates as "DD. Mmm YYYY" (German) or "YYYY-MM-DD".
func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Already ISO format
	if len(s) == 10 && s[4] == '-' {
		return s
	}
	// Try German month names: "25. Juni 2021"
	germanMonths := map[string]string{
		"Januar": "01", "Februar": "02", "März": "03", "April": "04",
		"Mai": "05", "Juni": "06", "Juli": "07", "August": "08",
		"September": "09", "Oktober": "10", "November": "11", "Dezember": "12",
	}
	parts := strings.Fields(s)
	if len(parts) == 3 {
		day := strings.TrimRight(parts[0], ".")
		mon := germanMonths[parts[1]]
		year := parts[2]
		if mon != "" && len(day) <= 2 && len(year) == 4 {
			if len(day) == 1 {
				day = "0" + day
			}
			return year + "-" + mon + "-" + day
		}
	}
	return s
}
