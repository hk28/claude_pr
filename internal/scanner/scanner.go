package scanner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hk28/prman/internal/config"
)

type ScanResult struct {
	SeriesSlug string
	MediaType  string // "audio" or "ebook"
	Number     int
	FolderPath string // absolute path in inbox
}

// ScanInbox scans the inbox directories for a series and returns matching issue folders.
func ScanInbox(cfg config.SeriesConfig, mainCfg config.MainConfig) ([]ScanResult, error) {
	var results []ScanResult

	for _, loc := range cfg.Locations {
		var inboxBase string
		switch loc.What {
		case "audio":
			inboxBase = filepath.Join(mainCfg.InboxAudio, cfg.Name)
		case "ebook":
			inboxBase = filepath.Join(mainCfg.InboxEbook, cfg.Name)
		default:
			continue
		}

		re, err := patternToRegexp(loc.ScanPattern)
		if err != nil {
			return nil, fmt.Errorf("bad scanpattern %q for series %s: %w", loc.ScanPattern, cfg.Name, err)
		}

		entries, err := os.ReadDir(inboxBase)
		if err != nil {
			if os.IsNotExist(err) {
				continue // inbox folder doesn't exist yet, that's fine
			}
			return nil, fmt.Errorf("reading inbox %s: %w", inboxBase, err)
		}

		for _, e := range entries {
			// Match both directories and files (strip extension for files)
			stem := e.Name()
			if !e.IsDir() {
				stem = strings.TrimSuffix(stem, filepath.Ext(stem))
			}
			name := strings.ToLower(stem)
			log.Printf("Scanning %s for series %s: checking name %q against pattern %q", inboxBase, cfg.Name, name, loc.ScanPattern)
			m := re.FindStringSubmatch(name)
			if m == nil {
				continue
			}
			num, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			results = append(results, ScanResult{
				SeriesSlug: cfg.SlugName,
				MediaType:  loc.What,
				Number:     num,
				FolderPath: filepath.Join(inboxBase, e.Name()),
			})
		}
	}

	return results, nil
}

// patternToRegexp converts a scanf-style pattern like "pr %.04d" to a regexp.
// Supported verbs: %d, %.Nd (zero-padded decimal).
// If the pattern contains no format verb, it is treated as a keyword: any
// filename that contains the keyword and has a number anywhere in it matches,
// and the first number found is used as the issue number.
func patternToRegexp(pattern string) (*regexp.Regexp, error) {
	const placeholder = "__NUM__"
	verbRe := regexp.MustCompile(`%\.?\d*d`)
	lower := strings.ToLower(pattern)
	if !verbRe.MatchString(lower) {
		// Keyword-only pattern: match anything containing the keyword plus a number.
		kw := regexp.QuoteMeta(strings.TrimSpace(lower))
		return regexp.Compile(`.*` + kw + `.*?(\d+).*`)
	}
	s := verbRe.ReplaceAllString(lower, placeholder)
	s = regexp.QuoteMeta(s)
	s = strings.ReplaceAll(s, placeholder, `(\d+)`)
	return regexp.Compile("^" + s + "$")
}
