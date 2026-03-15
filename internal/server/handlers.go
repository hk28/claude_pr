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
	mux.HandleFunc("GET /series/{slug}/copy-preview", s.handleCopyPreview)
	mux.HandleFunc("POST /series/{slug}/copy", s.handleCopy)
	mux.HandleFunc("POST /series/{slug}/issue/{num}/state", s.handleToggleState)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	return mux
}

func (s *Server) buildPageVM(viewMode, filterSlug, filterType string, onlyMissing bool) PageVM {
	var vms []SeriesVM
	for _, cfg := range s.series {
		st, _ := s.proc.State.Load(cfg.SlugName)
		vm := BuildSeriesVM(cfg, st, s.proc.Cache)
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
		viewMode = "medium"
	}
	filterSlug := r.URL.Query().Get("series")
	filterType := r.URL.Query().Get("type")
	onlyMissing := r.URL.Query().Get("missing") == "1"

	vm := s.buildPageVM(viewMode, filterSlug, filterType, onlyMissing)

	// HTMX partial swap for main content only
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Target") == "main-content" {
		s.renderPartial(w, "main_content", vm)
		return
	}
	s.renderPage(w, "index.html", vm)
}

func (s *Server) handleSeriesDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	cfg, ok := s.proc.SeriesBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	st, _ := s.proc.State.Load(slug)
	svm := BuildSeriesVM(cfg, st, s.proc.Cache)
	page := s.buildPageVM("details", slug, "", false)
	page.CurrentSeries = &svm
	s.renderPage(w, "series_detail.html", page)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.Scan(slug)
	s.renderPartial(w, "scan_preview", map[string]any{
		"Report": report,
		"Error":  errStr(err),
		"Slug":   slug,
	})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	report, err := s.proc.Update(slug)
	s.renderPartial(w, "update_result", map[string]any{
		"Report": report,
		"Error":  errStr(err),
		"Slug":   slug,
	})
}

func (s *Server) handleCopyPreview(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	actions, err := s.proc.CopyPreview(slug)
	s.renderPartial(w, "copy_preview", map[string]any{
		"Actions": actions,
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
	s.renderPartial(w, "copy_result", map[string]any{
		"Errors": errs,
		"Count":  len(actions) - len(errs),
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
