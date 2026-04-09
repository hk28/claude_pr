package server

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/hk28/prman/internal/config"
	"github.com/hk28/prman/internal/processor"
	"github.com/hk28/prman/internal/views"
)

type Server struct {
	proc      *processor.Processor
	series    []config.SeriesConfig
	main      config.MainConfig
	configDir string
}

func New(proc *processor.Processor, series []config.SeriesConfig, main config.MainConfig, configDir string) *Server {
	return &Server{proc: proc, series: series, main: main, configDir: configDir}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /series/{slug}", s.handleSeriesDetail)
	mux.HandleFunc("POST /series/{slug}/scan", s.handleScan)
	mux.HandleFunc("POST /series/{slug}/update", s.handleUpdate)
	mux.HandleFunc("POST /series/{slug}/refresh-metadata", s.handleRefreshMetadata)
	mux.HandleFunc("GET /series/{slug}/cards", s.handleSeriesCards)
	mux.HandleFunc("GET /series/{slug}/copy-preview", s.handleCopyPreview)
	mux.HandleFunc("POST /series/{slug}/copy", s.handleCopy)
	mux.HandleFunc("POST /series/{slug}/issue/{num}/state", s.handleToggleState)
	mux.HandleFunc("POST /series/{slug}/issue/{num}/set-output", s.handleSetOutput)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("GET /covers/", http.StripPrefix("/covers/", http.FileServer(http.Dir(s.proc.Cache.CoversDir))))
	mux.HandleFunc("POST /series/{slug}/clear-cache", s.handleClearCache)
	mux.HandleFunc("POST /series/{slug}/archive", s.handleArchive)
	mux.HandleFunc("GET /archive", s.handleArchiveList)
	mux.HandleFunc("POST /archive/{slug}/restore", s.handleRestore)
	mux.HandleFunc("POST /update-all", s.handleUpdateAll)
	mux.HandleFunc("POST /reload-config", s.handleReloadConfig)
	mux.HandleFunc("GET /missing-list", s.handleMissingList)
	mux.HandleFunc("GET /series/{slug}/patch-audio-metadata-preview", s.handlePatchAudioMetadataPreview)
	mux.HandleFunc("POST /series/{slug}/patch-audio-metadata", s.handlePatchAudioMetadata)
	return mux
	// return requestLogger(mux)
}

// requestLogger logs every non-static request so we can see what the server receives.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/static/") && !strings.HasPrefix(r.URL.Path, "/covers/") {
			log.Printf("→ %s %s", r.Method, r.URL.RequestURI())
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) buildPageVM(viewMode, filterSlug, filterType, sortOrder string, onlyMissing bool) views.PageVM {
	var vms []views.SeriesVM
	for _, cfg := range s.series {
		st, _ := s.proc.State.Load(cfg.SlugName)
		vm := BuildSeriesVM(cfg, st, s.proc.Cache)
		vm.ViewMode = viewMode
		vm.OnlyMissing = onlyMissing
		vm.FilterType = filterType
		vms = append(vms, vm)
	}
	if sortOrder == "date" {
		sort.Slice(vms, func(i, j int) bool {
			return vms[i].LatestReleaseDate > vms[j].LatestReleaseDate
		})
	} else {
		sortOrder = "name"
		sort.Slice(vms, func(i, j int) bool {
			return vms[i].Config.Name < vms[j].Config.Name
		})
	}
	var totalAudio, totalEbook int
	for _, vm := range vms {
		totalAudio += vm.MissingReleasedAudio
		totalEbook += vm.MissingReleasedEbook
	}
	return views.PageVM{
		Series:            vms,
		ViewMode:          viewMode,
		FilterSlug:        filterSlug,
		FilterType:        filterType,
		OnlyMissing:       onlyMissing,
		SortOrder:         sortOrder,
		TotalMissingAudio: totalAudio,
		TotalMissingEbook: totalEbook,
	}
}

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<div style="color:red">render error: %v</div>`, err)
	}
}

func (s *Server) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	series, err := config.LoadSeries(filepath.Join(s.configDir, "series"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.series = series
	s.proc.Series = series
	log.Printf("reloaded config: %d series", len(series))
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	viewMode := r.URL.Query().Get("view")
	if viewMode == "" {
		viewMode = "big"
	}
	filterSlug := r.URL.Query().Get("series")
	filterType := r.URL.Query().Get("type")
	onlyMissing := r.URL.Query().Get("missing") == "1"
	sortOrder := r.URL.Query().Get("sort")

	vm := s.buildPageVM(viewMode, filterSlug, filterType, sortOrder, onlyMissing)

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "main-content" {
		render(w, r, views.MainContent(vm))
		return
	}
	render(w, r, views.IndexPage(vm))
}

func (s *Server) handleSeriesCards(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	cfg, ok := s.proc.SeriesBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	viewMode := r.URL.Query().Get("view")
	if viewMode == "" {
		viewMode = "details"
	}
	filterType := r.URL.Query().Get("type")
	onlyMissing := r.URL.Query().Get("missing") == "1"
	st, _ := s.proc.State.Load(slug)
	svm := BuildSeriesVM(cfg, st, s.proc.Cache)
	svm.ViewMode = viewMode
	svm.FilterType = filterType
	svm.OnlyMissing = onlyMissing
	render(w, r, views.SeriesCards(svm))
}

func (s *Server) handleSeriesDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	cfg, ok := s.proc.SeriesBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	viewMode := r.URL.Query().Get("view")
	if viewMode == "" {
		viewMode = "details"
	}
	st, _ := s.proc.State.Load(slug)
	svm := BuildSeriesVM(cfg, st, s.proc.Cache)
	svm.ViewMode = viewMode
	filterType := r.URL.Query().Get("type")
	onlyMissing := r.URL.Query().Get("missing") == "1"
	sortOrder := r.URL.Query().Get("sort")
	svm.FilterType = filterType
	svm.OnlyMissing = onlyMissing
	page := s.buildPageVM(viewMode, slug, filterType, sortOrder, onlyMissing)
	page.CurrentSeries = &svm
	if r.Header.Get("HX-Request") == "true" {
		render(w, r, views.SeriesDetail(svm))
	} else {
		render(w, r, views.SeriesPage(page))
	}
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.Scan(slug)
	w.Header().Set("HX-Trigger", "seriesRefresh")
	render(w, r, views.ScanPreview(report, errStr(err)))
}

func (s *Server) handleUpdateAll(w http.ResponseWriter, r *http.Request) {
	var totalMarked int
	var fetchErrs []string
	var siteDown bool
	for _, cfg := range s.series {
		if siteDown {
			break
		}
		report, err := s.proc.UpdateQuick(cfg.SlugName)
		totalMarked += report.MarkedCount
		if err != nil {
			fetchErrs = append(fetchErrs, cfg.Name+": "+err.Error())
			siteDown = true
		} else if len(report.FetchErrors) > 0 {
			for _, fe := range report.FetchErrors {
				fetchErrs = append(fetchErrs, cfg.Name+" "+fe)
			}
			// A fetch error on the frontier likely means the site is unreachable
			siteDown = true
		}
	}
	w.Header().Set("HX-Trigger", "seriesRefresh")
	render(w, r, views.UpdateAllResult(totalMarked, fetchErrs))
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.Update(slug)
	w.Header().Set("HX-Trigger", "seriesRefresh")
	render(w, r, views.UpdateResult(report, errStr(err)))
}

func (s *Server) handleRefreshMetadata(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.RefreshMetadata(slug)
	render(w, r, views.RefreshResult(report, errStr(err)))
}

func (s *Server) handleClearCache(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	err := s.proc.ClearCache(slug)
	w.Header().Set("HX-Trigger", "seriesRefresh")
	render(w, r, views.ClearCacheResult(slug, errStr(err)))
}

func splitActions(all []processor.CopyAction) (pending, done []processor.CopyAction) {
	for _, a := range all {
		if a.AlreadyDone {
			done = append(done, a)
		} else {
			pending = append(pending, a)
		}
	}
	return
}

func (s *Server) handleCopyPreview(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	actions, err := s.proc.CopyPreview(slug)
	pending, done := splitActions(actions)
	render(w, r, views.CopyPreview(slug, pending, done, errStr(err)))
}

func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	actions, err := s.proc.CopyPreview(slug)
	var errs []string
	if err != nil {
		errs = []string{err.Error()}
	} else {
		errs = s.proc.CopyExecute(slug, actions)
	}
	pending, _ := splitActions(actions)
	w.Header().Set("HX-Trigger", "seriesRefresh")
	render(w, r, views.CopyResult(len(pending)-len(errs), errs))
}

func (s *Server) handleToggleState(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	numStr := r.PathValue("num")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		http.Error(w, "invalid issue number", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	stateName := r.FormValue("state")
	val := r.FormValue("value") == "true"

	if err := s.proc.State.SetState(slug, num, stateName, val); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cfg, ok := s.proc.SeriesBySlug(slug)
	if !ok {
		http.Error(w, "series not found", http.StatusNotFound)
		return
	}
	st, _ := s.proc.State.Load(slug)
	vm := BuildSeriesVM(cfg, st, s.proc.Cache)
	render(w, r, views.SidebarSeries(vm))
}

func (s *Server) handleSetOutput(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	numStr := r.PathValue("num")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		http.Error(w, "invalid issue number", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	mediaType := r.FormValue("mediaType")
	path := ""
	if r.FormValue("action") == "set" {
		path = "manual"
	}
	if err := s.proc.State.UpdateOutput(slug, num, mediaType, path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, ok := s.proc.SeriesBySlug(slug)
	if !ok {
		http.Error(w, "series not found", http.StatusNotFound)
		return
	}
	st, _ := s.proc.State.Load(slug)
	vm := BuildSeriesVM(cfg, st, s.proc.Cache)
	var issueVM views.IssueVM
	for _, iv := range vm.Issues {
		if iv.Number == num {
			issueVM = iv
			break
		}
	}
	render(w, r, views.IssueOutputCell(slug, issueVM))
}

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if err := s.proc.Archive(slug, s.configDir); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	series, err := config.LoadSeries(filepath.Join(s.configDir, "series"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.series = series
	s.proc.Series = series
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleArchiveList(w http.ResponseWriter, r *http.Request) {
	archived, err := config.LoadArchivedSeries(s.configDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "main-content" {
		render(w, r, views.ArchiveContent(archived))
		return
	}
	vm := s.buildPageVM("big", "", "", "name", false)
	render(w, r, views.ArchivePage(vm, archived))
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if err := s.proc.RestoreArchive(slug, s.configDir); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	series, err := config.LoadSeries(filepath.Join(s.configDir, "series"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.series = series
	s.proc.Series = series
	w.Header().Set("HX-Redirect", "/series/"+slug)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMissingList(w http.ResponseWriter, r *http.Request) {
	var sb strings.Builder
	sb.WriteString("Missing Items\n")
	sb.WriteString("Generated: " + time.Now().Format("2006-01-02") + "\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	totalAudio := 0
	totalEbook := 0

	for _, cfg := range s.series {
		st, _ := s.proc.State.Load(cfg.SlugName)
		vm := BuildSeriesVM(cfg, st, s.proc.Cache)

		if vm.MissingReleasedAudio == 0 && vm.MissingReleasedEbook == 0 {
			continue
		}

		sb.WriteString(cfg.Name + "\n")
		sb.WriteString(strings.Repeat("-", len(cfg.Name)) + "\n")

		for _, issue := range vm.Issues {
			if !issue.States["Released"] {
				continue
			}
			var missing []string
			if issue.HasAudio && issue.InboxAudio == "" && issue.OutputAudio == "" {
				missing = append(missing, "audio")
				totalAudio++
			}
			if issue.HasEbook && issue.InboxEbook == "" && issue.OutputEbook == "" {
				missing = append(missing, "ebook")
				totalEbook++
			}
			if len(missing) == 0 {
				continue
			}
			line := fmt.Sprintf("  #%d", issue.Number)
			if issue.Title != "" {
				line += " – " + issue.Title
			}
			line += " [" + strings.Join(missing, ", ") + "]\n"
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("=", 60) + "\n")
	sb.WriteString(fmt.Sprintf("Total missing: %d audio, %d ebook\n", totalAudio, totalEbook))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="missing-items.txt"`)
	fmt.Fprint(w, sb.String())
}

func (s *Server) handlePatchAudioMetadataPreview(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.PatchAudioMetadataPreview(slug)
	render(w, r, views.PatchAudioMetadataPreview(slug, report, errStr(err)))
}

func (s *Server) handlePatchAudioMetadata(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.PatchAudioMetadataPreview(slug)
	var errs []string
	if err != nil {
		errs = []string{err.Error()}
	} else {
		errs = s.proc.PatchAudioMetadataExecute(report.Actions)
	}
	changed := 0
	for _, a := range report.Actions {
		if a.Changed {
			changed++
		}
	}
	render(w, r, views.PatchAudioMetadataResult(changed-len(errs), errs))
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
