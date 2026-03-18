package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hk28/prman/internal/cache"
	"github.com/hk28/prman/internal/config"
	"github.com/hk28/prman/internal/processor"
	"github.com/hk28/prman/internal/scraper"
	"github.com/hk28/prman/internal/server"
	"github.com/hk28/prman/internal/state"
)

func main() {
	configDir := flag.String("config", "./config", "path to config directory")
	dataDir := flag.String("data", "./data", "path to data directory")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("creating data dir: %v", err)
	}
	logFile, err := os.OpenFile(filepath.Join(*dataDir, "log.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("opening log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))

	mainCfg, seriesCfgs, err := config.LoadAll(*configDir)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	fmt.Printf("prman loaded %d series:\n", len(seriesCfgs))
	for _, s := range seriesCfgs {
		fmt.Printf("  %s (%s)\n", s.Name, s.SlugName)
	}

	cacheDir := filepath.Join(*dataDir, "cache")
	stateDir := filepath.Join(*dataDir, "state")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Fatalf("creating cache dir: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Fatalf("creating state dir: %v", err)
	}

	c := cache.New(cacheDir)
	sm := state.New(stateDir)
	sc := scraper.New()
	proc := processor.New(mainCfg, seriesCfgs, c, sm, sc)

	tmpls, err := loadTemplates()
	if err != nil {
		log.Fatalf("loading templates: %v", err)
	}

	srv := server.New(proc, seriesCfgs, mainCfg, tmpls, *configDir)

	log.Printf("prman listening on http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

func loadTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		// dict creates a map from key-value pairs: {{dict "Key" val "Key2" val2}}
		"dict": func(pairs ...any) (map[string]any, error) {
			if len(pairs)%2 != 0 {
				return nil, fmt.Errorf("dict requires even number of arguments")
			}
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i < len(pairs); i += 2 {
				k, ok := pairs[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict key must be string")
				}
				m[k] = pairs[i+1]
			}
			return m, nil
		},
	}

	pattern := filepath.Join("templates", "**", "*.html")
	// Go's filepath.Glob doesn't support **, so collect manually
	var files []string
	err := filepath.WalkDir("templates", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".html" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = pattern

	if len(files) == 0 {
		return template.New("").Funcs(funcMap), nil
	}

	return template.New("").Funcs(funcMap).ParseFiles(files...)
}
