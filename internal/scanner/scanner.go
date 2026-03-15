package scanner

import (
	"fmt"
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
			if !e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
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
func patternToRegexp(pattern string) (*regexp.Regexp, error) {
	// Escape regex special characters except for the format verb
	// Replace %d and %.Nd with a capture group
	s := regexp.QuoteMeta(strings.ToLower(pattern))
	// QuoteMeta escapes %, so we look for literal \%
	// Replace \%\.\d+d and \%d with number capture group
	padRe := regexp.MustCompile(`\\%\\\.0*\d*d`)
	s = padRe.ReplaceAllString(s, `(\d+)`)
	s = strings.ReplaceAll(s, `\%d`, `(\d+)`)
	return regexp.Compile("^" + s + "$")
}
