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
	Author      string
	ReleaseDate string
	SubSeries   string
	States      map[string]bool
	StateNames  []string // ordered list from series config
	InboxAudio  string
	InboxEbook  string
	OutputAudio string
	OutputEbook string
	CacheExists bool
}

// SeriesVM is the per-series view model.
type SeriesVM struct {
	Config        config.SeriesConfig
	Issues        []IssueVM
	MissingAudio  int
	MissingEbook  int
	TotalReleased int
}

// PageVM is the top-level view model for all pages (index + series detail).
type PageVM struct {
	Series        []SeriesVM
	CurrentSeries *SeriesVM // non-nil on series detail page
	ViewMode      string    // "big", "medium", "details"
	FilterSlug    string
	FilterType    string // "audio", "ebook", ""
	OnlyMissing   bool
}

// cacheGetter is satisfied by *cache.Cache.
type cacheGetter interface {
	Get(slug string, number int) (cache.ScrapedIssue, bool)
}

// BuildSeriesVM constructs a SeriesVM from config + state + cache.
func BuildSeriesVM(cfg config.SeriesConfig, st state.SeriesState, c cacheGetter) SeriesVM {
	vm := SeriesVM{Config: cfg}

	// Collect and sort issue numbers
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
		}
		if issue, ok := c.Get(cfg.SlugName, num); ok {
			iv.Title = issue.Title
			iv.Author = issue.Author
			iv.ReleaseDate = issue.ReleaseDate
			iv.SubSeries = issue.SubSeries
			iv.CacheExists = true
		}

		if is.InboxAudio != "" && is.OutputAudio == "" {
			vm.MissingAudio++
		}
		if is.InboxEbook != "" && is.OutputEbook == "" {
			vm.MissingEbook++
		}
		if is.States["Released"] {
			vm.TotalReleased++
		}
		vm.Issues = append(vm.Issues, iv)
	}
	return vm
}
