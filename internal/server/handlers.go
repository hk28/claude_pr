package server

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/hk28/prman/internal/config"
	"github.com/hk28/prman/internal/processor"
)

type Server struct {
	proc   *processor.Processor
	tmpls  *template.Template
	series []config.SeriesConfig
	main   config.MainConfig
}

func New(proc *processor.Processor, series []config.SeriesConfig, main config.MainConfig, tmpls *template.Template) *Server {
	return &Server{proc: proc, tmpls: tmpls, series: series, main: main}
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
	return mux
}

func (s *Server) buildPageVM(viewMode, filterSlug, filterType string, onlyMissing bool) PageVM {
	var vms []SeriesVM
	for _, cfg := range s.series {
		st, _ := s.proc.State.Load(cfg.SlugName)
		vm := BuildSeriesVM(cfg, st, s.proc.Cache)
		vm.ViewMode = viewMode
		vms = append(vms, vm)
	}
	return PageVM{
		Series:      vms,
		ViewMode:    viewMode,
		FilterSlug:  filterSlug,
		FilterType:  filterType,
		OnlyMissing: onlyMissing,
	}
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

	vm := s.buildPageVM(viewMode, filterSlug, filterType, onlyMissing)

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "main-content" {
		s.renderPartial(w, "main_content", vm)
		return
	}
	s.renderPage(w, "index.html", vm)
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
	st, _ := s.proc.State.Load(slug)
	svm := BuildSeriesVM(cfg, st, s.proc.Cache)
	svm.ViewMode = viewMode
	s.renderPartial(w, "series_cards", svm)
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
	page := s.buildPageVM(viewMode, slug, "", false)
	page.CurrentSeries = &svm
	s.renderPage(w, "series_detail.html", page)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.Scan(slug)
	w.Header().Set("HX-Trigger", "seriesRefresh")
	s.renderPartial(w, "scan_preview", map[string]any{
		"Report": report,
		"Error":  errStr(err),
		"Slug":   slug,
	})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.Update(slug)
	w.Header().Set("HX-Trigger", "seriesRefresh")
	s.renderPartial(w, "update_result", map[string]any{
		"Report": report,
		"Error":  errStr(err),
		"Slug":   slug,
	})
}

func (s *Server) handleRefreshMetadata(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.RefreshMetadata(slug)
	s.renderPartial(w, "refresh_result", map[string]any{
		"Report": report,
		"Error":  errStr(err),
		"Slug":   slug,
	})
}

func (s *Server) handleClearCache(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	err := s.proc.ClearCache(slug)
	s.renderPartial(w, "clear_cache_result", map[string]any{
		"Error": errStr(err),
		"Slug":  slug,
	})
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
	s.renderPartial(w, "copy_preview", map[string]any{
		"Actions": pending,
		"Done":    done,
		"Error":   errStr(err),
		"Slug":    slug,
	})
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
	s.renderPartial(w, "copy_result", map[string]any{
		"Errors": errs,
		"Count":  len(pending) - len(errs),
		"Slug":   slug,
	})
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
	s.renderPartial(w, "sidebar_series", vm)
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
	var issueVM IssueVM
	for _, iv := range vm.Issues {
		if iv.Number == num {
			issueVM = iv
			break
		}
	}
	s.renderPartial(w, "issue_output_cell", map[string]any{
		"Slug":  slug,
		"Issue": issueVM,
	})
}

func (s *Server) renderPage(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpls.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpls.ExecuteTemplate(w, name, data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<div class="error">Template error: %v</div>`, err)
	}
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
