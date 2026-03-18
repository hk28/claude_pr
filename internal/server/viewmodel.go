package server

import (
	"fmt"
	"sort"

	"github.com/hk28/prman/internal/cache"
	"github.com/hk28/prman/internal/config"
	"github.com/hk28/prman/internal/state"
	"github.com/hk28/prman/internal/views"
)

// cacheGetter is satisfied by *cache.Cache.
type cacheGetter interface {
	Get(slug string, number int) (cache.ScrapedIssue, bool)
}

// BuildSeriesVM constructs a SeriesVM from config + state + cache.
func BuildSeriesVM(cfg config.SeriesConfig, st state.SeriesState, c cacheGetter) views.SeriesVM {
	vm := views.SeriesVM{Config: cfg}

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
		iv := views.IssueVM{
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
				vm.CoverURL = issue.CoverURL
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
