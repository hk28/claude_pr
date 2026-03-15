package processor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hk28/prman/internal/cache"
	"github.com/hk28/prman/internal/config"
	"github.com/hk28/prman/internal/metadata"
	"github.com/hk28/prman/internal/scanner"
	"github.com/hk28/prman/internal/scraper"
	"github.com/hk28/prman/internal/state"
)

type Processor struct {
	Main    config.MainConfig
	Series  []config.SeriesConfig
	Cache   *cache.Cache
	State   *state.Manager
	Scraper *scraper.Scraper
}

func New(main config.MainConfig, series []config.SeriesConfig, c *cache.Cache, s *state.Manager, sc *scraper.Scraper) *Processor {
	return &Processor{Main: main, Series: series, Cache: c, State: s, Scraper: sc}
}

func (p *Processor) SeriesBySlug(slug string) (config.SeriesConfig, bool) {
	for _, s := range p.Series {
		if s.SlugName == slug {
			return s, true
		}
	}
	return config.SeriesConfig{}, false
}

// ScanReport summarizes the result of a Scan operation.
type ScanReport struct {
	Found    []scanner.ScanResult
	Fetched  []int  // issue numbers newly cached
	Errors   []string
}

// Scan scans the inbox for a series, fetches missing metadata, and updates state.
func (p *Processor) Scan(seriesSlug string) (ScanReport, error) {
	cfg, ok := p.SeriesBySlug(seriesSlug)
	if !ok {
		return ScanReport{}, fmt.Errorf("unknown series: %s", seriesSlug)
	}

	var report ScanReport

	results, err := scanner.ScanInbox(cfg, p.Main)
	if err != nil {
		return report, err
	}
	report.Found = results

	for _, r := range results {
		// Ensure state entry exists
		if err := p.State.EnsureIssue(seriesSlug, r.Number, cfg.States); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("state for #%d: %v", r.Number, err))
			continue
		}
		// Update inbox path in state
		if err := p.State.UpdateInbox(seriesSlug, r.Number, r.MediaType, r.FolderPath); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("inbox state for #%d: %v", r.Number, err))
		}
		// Fetch metadata if not cached
		if !p.Cache.Exists(seriesSlug, r.Number) {
			issue, err := p.Scraper.FetchIssue(cfg, r.Number)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("fetch #%d: %v", r.Number, err))
				continue
			}
			if err := p.Cache.Set(seriesSlug, issue); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("cache #%d: %v", r.Number, err))
				continue
			}
			report.Fetched = append(report.Fetched, r.Number)
		}
	}

	return report, nil
}

// UpdateReport summarizes the result of an Update operation.
type UpdateReport struct {
	LatestIssue int
	MarkedCount int
}

// Update calculates which issues should be released and marks them in state.
func (p *Processor) Update(seriesSlug string) (UpdateReport, error) {
	cfg, ok := p.SeriesBySlug(seriesSlug)
	if !ok {
		return UpdateReport{}, fmt.Errorf("unknown series: %s", seriesSlug)
	}

	latest := cfg.Latest
	if latest == 0 {
		latest = calcLatest(cfg)
	}

	var count int
	for n := cfg.ScanFrom; n <= latest; n++ {
		if err := p.State.EnsureIssue(seriesSlug, n, cfg.States); err != nil {
			return UpdateReport{}, err
		}
		if err := p.State.SetState(seriesSlug, n, "Released", true); err != nil {
			return UpdateReport{}, err
		}
		count++
	}

	return UpdateReport{LatestIssue: latest, MarkedCount: count}, nil
}

// calcLatest calculates the latest released issue number based on anchor + interval.
func calcLatest(cfg config.SeriesConfig) int {
	anchor, err := time.Parse("2006-01-02", cfg.Anchor.Date)
	if err != nil {
		return cfg.Anchor.Number
	}
	days := time.Since(anchor).Hours() / 24
	return cfg.Anchor.Number + int(days)/cfg.Interval
}

// CopyAction describes a single inbox→output copy operation.
type CopyAction struct {
	Number    int
	MediaType string // "audio" or "ebook"
	SrcPath   string
	DstDir    string // destination directory (not yet created)
	DstName   string // rendered folder name
}

// CopyPreview returns the list of copy actions that would be executed, without doing them.
func (p *Processor) CopyPreview(seriesSlug string) ([]CopyAction, error) {
	cfg, ok := p.SeriesBySlug(seriesSlug)
	if !ok {
		return nil, fmt.Errorf("unknown series: %s", seriesSlug)
	}

	s, err := p.State.Load(seriesSlug)
	if err != nil {
		return nil, err
	}

	var actions []CopyAction
	for _, is := range s.Issues {
		issue, cached := p.Cache.Get(seriesSlug, is.Number)

		tdata := config.TemplateData{Number: is.Number}
		if cached {
			tdata.Title = issue.Title
			tdata.SubSeries = issue.SubSeries
			tdata.Author = issue.Author
		}

		subdir, err := config.RenderTemplate(cfg.Subdir, tdata)
		if err != nil {
			subdir = fmt.Sprintf("issue-%d", is.Number)
		}

		if is.InboxAudio != "" && is.OutputAudio == "" {
			actions = append(actions, CopyAction{
				Number:    is.Number,
				MediaType: "audio",
				SrcPath:   is.InboxAudio,
				DstDir:    filepath.Join(p.Main.OutputAudio, cfg.Name),
				DstName:   subdir,
			})
		}
		if is.InboxEbook != "" && is.OutputEbook == "" {
			actions = append(actions, CopyAction{
				Number:    is.Number,
				MediaType: "ebook",
				SrcPath:   is.InboxEbook,
				DstDir:    filepath.Join(p.Main.OutputEbook, cfg.Name),
				DstName:   subdir,
			})
		}
	}

	return actions, nil
}

// CopyExecute performs the copy actions and writes metadata files.
func (p *Processor) CopyExecute(seriesSlug string, actions []CopyAction) []string {
	cfg, ok := p.SeriesBySlug(seriesSlug)
	var errs []string
	if !ok {
		return []string{fmt.Sprintf("unknown series: %s", seriesSlug)}
	}

	for _, a := range actions {
		dst := filepath.Join(a.DstDir, a.DstName)
		if err := os.MkdirAll(dst, 0755); err != nil {
			errs = append(errs, fmt.Sprintf("#%d: mkdir: %v", a.Number, err))
			continue
		}

		// Copy all files from src to dst
		if err := copyDir(a.SrcPath, dst); err != nil {
			errs = append(errs, fmt.Sprintf("#%d: copy: %v", a.Number, err))
			continue
		}

		// Write metadata
		issue, hasCached := p.Cache.Get(seriesSlug, a.Number)
		if hasCached {
			switch a.MediaType {
			case "audio":
				m, err := metadata.GenerateAudio(cfg, issue)
				if err == nil {
					b, err := metadata.MarshalAudio(m)
					if err == nil {
						_ = os.WriteFile(filepath.Join(dst, "metadata.json"), b, 0644)
					}
				}
			case "ebook":
				opf, err := metadata.GenerateOPF(cfg, issue, a.Number)
				if err == nil {
					_ = os.WriteFile(filepath.Join(dst, fmt.Sprintf("%d.opf", a.Number)), []byte(opf), 0644)
				}
			}
		}

		// Update state
		_ = p.State.UpdateOutput(seriesSlug, a.Number, a.MediaType, dst)
	}

	return errs
}

// copyDir copies all files from src directory into dst directory (non-recursive).
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue // skip subdirs
		}
		srcFile := filepath.Join(src, e.Name())
		dstFile := filepath.Join(dst, e.Name())
		if err := copyFile(srcFile, dstFile); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
