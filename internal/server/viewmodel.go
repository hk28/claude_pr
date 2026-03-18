package server

import (
	"fmt"
	"sort"

	"github.com/hk28/prman/internal/cache"
	"github.com/hk28/prman/internal/config"
	"github.com/hk28/prman/internal/state"
)

// IssueVM is the per-issue view model for templates.
type IssueVM struct {
	Number      int
	Title       string
	Description string
	Author      string
	ReleaseDate string
	SubSeries   string
	CoverURL    string
	SourceURL   string // Perrypedia page URL
	States      map[string]bool
	StateNames  []string // ordered list from series config
	InboxAudio  string
	InboxEbook  string
	OutputAudio string
	OutputEbook string
	CacheExists bool
	HasAudio    bool
	HasEbook    bool
}

// IssueCardVM wraps an IssueVM with series context needed by issue card templates.
type IssueCardVM struct {
	Issue IssueVM
	Slug  string
}

// SeriesVM is the per-series view model.
type SeriesVM struct {
	Config               config.SeriesConfig
	Issues               []IssueVM
	CoverURL             string // cover of the latest issue that has one
	LatestReleaseDate    string // release date of the most recent cached issue
	MissingAudio         int   // in inbox, not yet copied to output
	MissingEbook         int
	MissingReleasedAudio int // released but not yet in inbox
	MissingReleasedEbook int
	TotalReleased        int
	ViewMode             string // inherited from page context
}

// PageVM is the top-level view model for all pages (index + series detail).
type PageVM struct {
	Series            []SeriesVM
	CurrentSeries     *SeriesVM // non-nil on series detail page
	ViewMode          string    // "big", "medium", "details"
	FilterSlug        string
	FilterType        string // "audio", "ebook", ""
	OnlyMissing       bool
	SortOrder         string // "name" or "date"
	TotalMissingAudio int
	TotalMissingEbook int
}

// cacheGetter is satisfied by *cache.Cache.
type cacheGetter interface {
	Get(slug string, number int) (cache.ScrapedIssue, bool)
}

// BuildSeriesVM constructs a SeriesVM from config + state + cache.
func BuildSeriesVM(cfg config.SeriesConfig, st state.SeriesState, c cacheGetter) SeriesVM {
	vm := SeriesVM{Config: cfg}

	hasAudio := containsType(cfg.Types, "audio")
	hasEbook := containsType(cfg.Types, "ebook")

	var sorted []int
	seen := map[int]bool{}
	for _, is := range st.Issues {
		if !seen[is.Number] {
			seen[is.Number] = true
			sorted = append(sorted, is.Number)
		}
	}
	sort.Ints(sorted)

	for _, num := range sorted {
		is := st.Issues[fmt.Sprintf("%d", num)]
		if is == nil {
			continue
		}
		iv := IssueVM{
			Number:      is.Number,
			States:      is.States,
			StateNames:  cfg.States,
			InboxAudio:  is.InboxAudio,
			InboxEbook:  is.InboxEbook,
			OutputAudio: is.OutputAudio,
			OutputEbook: is.OutputEbook,
			HasAudio:    hasAudio,
			HasEbook:    hasEbook,
		}
		if issue, ok := c.Get(cfg.SlugName, num); ok {
			iv.Title = issue.Title
			iv.Description = issue.Description
			iv.Author = issue.Author
			iv.ReleaseDate = issue.ReleaseDate
			iv.SubSeries = issue.SubSeries
			iv.CoverURL = issue.CoverURL
			iv.SourceURL = issue.SourceURL
			iv.CacheExists = true
			if issue.CoverURL != "" {
				vm.CoverURL = issue.CoverURL // keeps updating → ends up as latest issue's cover
			}
			if issue.ReleaseDate != "" {
				vm.LatestReleaseDate = issue.ReleaseDate
			}
		}

		released := is.States["Released"]
		if released {
			vm.TotalReleased++
			if hasAudio && is.InboxAudio == "" && is.OutputAudio == "" {
				vm.MissingReleasedAudio++
			}
			if hasEbook && is.InboxEbook == "" && is.OutputEbook == "" {
				vm.MissingReleasedEbook++
			}
		}
		if is.InboxAudio != "" && is.OutputAudio == "" {
			vm.MissingAudio++
		}
		if is.InboxEbook != "" && is.OutputEbook == "" {
			vm.MissingEbook++
		}
		vm.Issues = append(vm.Issues, iv)
	}
	return vm
}

func containsType(types []string, t string) bool {
	for _, v := range types {
		if v == t {
			return true
		}
	}
	return false
}
