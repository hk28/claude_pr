package views

import "github.com/hk28/prman/internal/config"

// IssueVM is the per-issue view model.
type IssueVM struct {
	Number      int
	Title       string
	Description string
	Author      string
	ReleaseDate string
	SubSeries   string
	CoverURL    string
	SourceURL   string
	States      map[string]bool
	StateNames  []string
	InboxAudio  string
	InboxEbook  string
	OutputAudio string
	OutputEbook string
	CacheExists bool
	HasAudio    bool
	HasEbook    bool
}

// SeriesVM is the per-series view model.
type SeriesVM struct {
	Config               config.SeriesConfig
	Issues               []IssueVM
	CoverURL             string
	LatestReleaseDate    string
	MissingAudio         int
	MissingEbook         int
	MissingReleasedAudio int
	MissingReleasedEbook int
	TotalReleased        int
	ViewMode             string
}

// PageVM is the top-level view model for all pages.
type PageVM struct {
	Series            []SeriesVM
	CurrentSeries     *SeriesVM
	ViewMode          string
	FilterSlug        string
	FilterType        string
	OnlyMissing       bool
	SortOrder         string
	TotalMissingAudio int
	TotalMissingEbook int
}
