package processor

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

// scrapeAndCache fetches issue metadata, downloads the cover image locally,
// and writes the JSON cache entry. Returns the saved issue.
func (p *Processor) scrapeAndCache(slug string, cfg config.SeriesConfig, number int) (cache.ScrapedIssue, error) {
	issue, err := p.Scraper.FetchIssue(cfg, number)
	if err != nil {
		return issue, err
	}
	if issue.CoverURL != "" {
		issue.CoverURL = p.downloadCover(slug, number, issue.CoverURL)
	}
	return issue, p.Cache.Set(slug, issue)
}

// fullResImageURL converts a Perrypedia thumbnail URL to its full-resolution equivalent
// by removing the /thumb/ path component and the trailing /NNpx-filename segment.
// e.g. .../images/thumb/c/c2/PR3360.jpg/400px-PR3360.jpg → .../images/c/c2/PR3360.jpg
func fullResImageURL(rawURL string) string {
	const thumbMarker = "/images/thumb/"
	idx := strings.Index(rawURL, thumbMarker)
	if idx < 0 {
		return rawURL
	}
	rest := rawURL[idx+len(thumbMarker):]
	lastSlash := strings.LastIndex(rest, "/")
	if lastSlash < 0 {
		return rawURL
	}
	return rawURL[:idx] + "/images/" + rest[:lastSlash]
}

// upscaleThumbURL replaces the Perrypedia thumbnail size prefix (e.g. /200px-)
// with /400px- to fetch a higher-resolution version.
func upscaleThumbURL(rawURL string) string {
	lastSlash := strings.LastIndex(rawURL, "/")
	if lastSlash < 0 {
		return rawURL
	}
	seg := rawURL[lastSlash+1:] // e.g. "200px-PR3300.jpg"
	pxIdx := strings.Index(seg, "px-")
	if pxIdx <= 0 {
		return rawURL
	}
	for _, c := range seg[:pxIdx] {
		if c < '0' || c > '9' {
			return rawURL
		}
	}
	return rawURL[:lastSlash+1] + "400px-" + seg[pxIdx+3:]
}

// downloadCover fetches the remote cover image and stores it under CoversDir.
// Tries in order: 400px upscaled → full-resolution → original thumbnail.
// Returns the local URL path ("/covers/<slug>/<n>.<ext>") on success,
// or the original remote URL if all attempts fail.
func (p *Processor) downloadCover(slug string, number int, remoteURL string) string {
	originalURL := remoteURL

	// 1. Try 400px upscaled thumbnail
	upscaled := upscaleThumbURL(remoteURL)
	if upscaled != originalURL {
		log.Printf("downloading cover for %s #%d: %s", slug, number, upscaled)
		if local, ok := p.fetchAndSaveCover(slug, number, upscaled); ok {
			return local
		}
	}

	// 2. Try full-resolution image (strips /thumb/ path)
	fullRes := fullResImageURL(originalURL)
	if fullRes != originalURL {
		log.Printf("cover %s #%d: trying full resolution: %s", slug, number, fullRes)
		if local, ok := p.fetchAndSaveCover(slug, number, fullRes); ok {
			return local
		}
	}

	// 3. Fall back to original thumbnail size
	log.Printf("cover %s #%d: falling back to original: %s", slug, number, originalURL)
	if local, ok := p.fetchAndSaveCover(slug, number, originalURL); ok {
		return local
	}
	return originalURL
}

// fetchAndSaveCover downloads a single URL and saves it to CoversDir.
// Returns the local path and true on success.
func (p *Processor) fetchAndSaveCover(slug string, number int, remoteURL string) (string, bool) {
	u, err := url.Parse(remoteURL)
	if err != nil {
		log.Printf("cover %s #%d: bad URL: %v", slug, number, err)
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	if ext == "" {
		ext = ".jpg"
	}

	req, _ := http.NewRequest("GET", remoteURL, nil)
	req.Header.Set("User-Agent", "prman/1.0 (personal media manager)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("cover %s #%d: fetch error: %v", slug, number, err)
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("cover %s #%d: HTTP %d", slug, number, resp.StatusCode)
		return "", false
	}

	dir := filepath.Join(p.Cache.CoversDir, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("cover %s #%d: mkdir: %v", slug, number, err)
		return "", false
	}

	dst := filepath.Join(dir, fmt.Sprintf("%d%s", number, ext))
	f, err := os.Create(dst)
	if err != nil {
		log.Printf("cover %s #%d: create file: %v", slug, number, err)
		return "", false
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		log.Printf("cover %s #%d: write: %v", slug, number, err)
		os.Remove(dst)
		return "", false
	}
	log.Printf("cover %s #%d saved → %s", slug, number, dst)
	return fmt.Sprintf("/covers/%s/%d%s", slug, number, ext), true
}

// ClearCache deletes the JSON metadata cache and cover images for a series.
func (p *Processor) ClearCache(slug string) error {
	return p.Cache.DeleteAll(slug)
}

// ScanReport summarizes the result of a Scan operation.
type ScanReport struct {
	Found       []scanner.ScanResult
	Fetched     []int // issue numbers newly cached
	Errors      []string
	ScannedDirs []string // inbox directories that were searched
}

// Scan scans the inbox for a series, fetches missing metadata, and updates state.
func (p *Processor) Scan(seriesSlug string) (ScanReport, error) {
	cfg, ok := p.SeriesBySlug(seriesSlug)
	if !ok {
		return ScanReport{}, fmt.Errorf("unknown series: %s", seriesSlug)
	}

	var report ScanReport

	for _, loc := range cfg.Locations {
		var dir string
		switch loc.What {
		case "audio":
			dir = filepath.Join(p.Main.InboxAudio, cfg.Name)
		case "ebook":
			dir = filepath.Join(p.Main.InboxEbook, cfg.Name)
		}
		if dir != "" {
			report.ScannedDirs = append(report.ScannedDirs, dir)
		}
	}

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
			if _, err := p.scrapeAndCache(seriesSlug, cfg, r.Number); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("fetch #%d: %v", r.Number, err))
				continue
			}
			report.Fetched = append(report.Fetched, r.Number)
		}
	}

	// Clear stale inbox paths: paths recorded in state that no longer exist on disk.
	p.clearStaleInbox(seriesSlug)

	return report, nil
}

// clearStaleInbox removes InboxAudio/InboxEbook from state for any issue
// whose recorded path no longer exists on disk.
func (p *Processor) clearStaleInbox(seriesSlug string) {
	st, err := p.State.Load(seriesSlug)
	if err != nil {
		return
	}
	for _, is := range st.Issues {
		if is.InboxAudio != "" {
			if _, err := os.Stat(is.InboxAudio); os.IsNotExist(err) {
				_ = p.State.ClearInbox(seriesSlug, is.Number, "audio")
			}
		}
		if is.InboxEbook != "" {
			if _, err := os.Stat(is.InboxEbook); os.IsNotExist(err) {
				_ = p.State.ClearInbox(seriesSlug, is.Number, "ebook")
			}
		}
	}
}

// UpdateReport summarizes the result of an Update operation.
type UpdateReport struct {
	LatestIssue int
	MarkedCount int
	Fetched     []int // issue numbers newly fetched during update
	FetchErrors []string
}

// Update calculates which issues should be released, marks them in state,
// and auto-fetches metadata for newly released issues that are not yet cached.
func (p *Processor) Update(seriesSlug string) (UpdateReport, error) {
	cfg, ok := p.SeriesBySlug(seriesSlug)
	if !ok {
		return UpdateReport{}, fmt.Errorf("unknown series: %s", seriesSlug)
	}

	latest := cfg.Latest
	if latest == 0 {
		latest = calcLatest(cfg)
	}

	var report UpdateReport
	report.LatestIssue = latest

	for n := cfg.ScanFrom; n <= latest; n++ {
		if err := p.State.EnsureIssue(seriesSlug, n, cfg.States); err != nil {
			return report, err
		}
		if err := p.State.SetState(seriesSlug, n, "Released", true); err != nil {
			return report, err
		}
		report.MarkedCount++

		// Auto-fetch metadata for this issue if not yet cached
		if !p.Cache.Exists(seriesSlug, n) {
			if _, err := p.scrapeAndCache(seriesSlug, cfg, n); err != nil {
				report.FetchErrors = append(report.FetchErrors, fmt.Sprintf("#%d: %v", n, err))
				continue
			}
			report.Fetched = append(report.Fetched, n)
		}
	}

	return report, nil
}

// RefreshReport summarizes the result of a RefreshMetadata operation.
type RefreshReport struct {
	Fetched []int
	Errors  []string
}

// RefreshMetadata re-fetches metadata for all released issues, overwriting the cache.
func (p *Processor) RefreshMetadata(seriesSlug string) (RefreshReport, error) {
	cfg, ok := p.SeriesBySlug(seriesSlug)
	if !ok {
		return RefreshReport{}, fmt.Errorf("unknown series: %s", seriesSlug)
	}

	st, err := p.State.Load(seriesSlug)
	if err != nil {
		return RefreshReport{}, err
	}

	var report RefreshReport
	for _, is := range st.Issues {
		if !is.States["Released"] {
			continue
		}
		if _, err := p.scrapeAndCache(seriesSlug, cfg, is.Number); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("#%d: %v", is.Number, err))
			continue
		}
		report.Fetched = append(report.Fetched, is.Number)
	}
	return report, nil
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
	Number      int
	MediaType   string // "audio" or "ebook"
	SrcPath     string
	DstDir      string // destination directory (not yet created)
	DstName     string // rendered folder name
	AlreadyDone bool   // inbox present but already copied to output — display only, not executed
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

		if is.InboxAudio != "" {
			if is.OutputAudio != "" {
				actions = append(actions, CopyAction{
					Number: is.Number, MediaType: "audio",
					SrcPath: is.InboxAudio, DstDir: is.OutputAudio, AlreadyDone: true,
				})
			} else if _, err := os.Stat(is.InboxAudio); err == nil {
				actions = append(actions, CopyAction{
					Number:    is.Number,
					MediaType: "audio",
					SrcPath:   is.InboxAudio,
					DstDir:    p.Main.OutputAudio,
					DstName:   subdir,
				})
			} // else: source gone — skip silently
		}
		if is.InboxEbook != "" {
			if is.OutputEbook != "" {
				actions = append(actions, CopyAction{
					Number: is.Number, MediaType: "ebook",
					SrcPath: is.InboxEbook, DstDir: is.OutputEbook, AlreadyDone: true,
				})
			} else if _, err := os.Stat(is.InboxEbook); err == nil {
				actions = append(actions, CopyAction{
					Number:    is.Number,
					MediaType: "ebook",
					SrcPath:   is.InboxEbook,
					DstDir:    p.Main.OutputEbook,
					DstName:   subdir,
				})
			} // else: source gone — skip silently
		}
	}

	return actions, nil
}

// CopyExecute moves files from inbox to output and writes metadata.
// Ebooks: only the .epub file is moved into DstDir/DstName/.
// Audio: the entire source folder is renamed/moved to DstDir/DstName.
func (p *Processor) CopyExecute(seriesSlug string, actions []CopyAction) []string {
	cfg, ok := p.SeriesBySlug(seriesSlug)
	var errs []string
	if !ok {
		return []string{fmt.Sprintf("unknown series: %s", seriesSlug)}
	}

	for _, a := range actions {
		if a.AlreadyDone {
			continue
		}
		dst := filepath.Join(a.DstDir, a.DstName)

		switch a.MediaType {
		case "ebook":
			if err := os.MkdirAll(dst, 0755); err != nil {
				errs = append(errs, fmt.Sprintf("#%d: mkdir: %v", a.Number, err))
				continue
			}
			if _, err := moveEpub(a.SrcPath, dst); err != nil {
				errs = append(errs, fmt.Sprintf("#%d: move epub: %v", a.Number, err))
				continue
			}
			issue, hasCached := p.Cache.Get(seriesSlug, a.Number)
			if hasCached {
				opf, err := metadata.GenerateOPF(cfg, issue, a.Number)
				if err == nil {
					_ = os.WriteFile(filepath.Join(dst, fmt.Sprintf("%d.opf", a.Number)), []byte(opf), 0644)
				}
			}

		case "audio":
			if err := os.MkdirAll(a.DstDir, 0755); err != nil {
				errs = append(errs, fmt.Sprintf("#%d: mkdir: %v", a.Number, err))
				continue
			}
			if err := moveDir(a.SrcPath, dst); err != nil {
				errs = append(errs, fmt.Sprintf("#%d: move audio: %v", a.Number, err))
				continue
			}
			issue, hasCached := p.Cache.Get(seriesSlug, a.Number)
			if hasCached {
				m, err := metadata.GenerateAudio(cfg, issue)
				if err == nil {
					b, err := metadata.MarshalAudio(m)
					if err == nil {
						_ = os.WriteFile(filepath.Join(dst, "metadata.json"), b, 0644)
					}
				}
			}
		}

		_ = p.State.UpdateOutput(seriesSlug, a.Number, a.MediaType, dst)
		_ = p.State.ClearInbox(seriesSlug, a.Number, a.MediaType)
	}

	return errs
}

// moveEpub moves the first .epub file found at src (file or folder) into dstDir.
// If src is a folder, it is removed after the epub is extracted.
// Returns the destination file path.
func moveEpub(src, dstDir string) (string, error) {
	epubFile := src
	srcIsDir := false
	if strings.ToLower(filepath.Ext(src)) != ".epub" {
		// src is a directory — find first epub inside
		srcIsDir = true
		entries, err := os.ReadDir(src)
		if err != nil {
			return "", err
		}
		epubFile = ""
		for _, e := range entries {
			if !e.IsDir() && strings.ToLower(filepath.Ext(e.Name())) == ".epub" {
				epubFile = filepath.Join(src, e.Name())
				break
			}
		}
		if epubFile == "" {
			return "", fmt.Errorf("no epub file found in %s", src)
		}
	}
	dst := filepath.Join(dstDir, filepath.Base(epubFile))
	if err := os.Rename(epubFile, dst); err != nil {
		// Cross-filesystem fallback
		if err := copyFile(epubFile, dst); err != nil {
			return "", err
		}
		if err := os.Remove(epubFile); err != nil {
			return "", err
		}
	}
	// Remove the now-empty source folder so the scanner won't pick it up again
	if srcIsDir {
		os.RemoveAll(src)
	}
	return dst, nil
}

// moveDir moves src directory to dst by rename, falling back to recursive copy+remove
// for cross-filesystem moves.
func moveDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyDirAll(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyDirAll(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDirAll(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
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
