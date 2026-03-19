package main

import (
	"flag"
	"fmt"
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

//go:generate templ generate

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

	srv := server.New(proc, seriesCfgs, mainCfg, *configDir)

	log.Printf("prman listening on http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
