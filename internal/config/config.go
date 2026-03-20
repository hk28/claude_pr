package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

type MainConfig struct {
	InboxAudio  string `yaml:"inbox_audio"`
	InboxEbook  string `yaml:"inbox_ebook"`
	OutputAudio string `yaml:"output_audio"`
	OutputEbook string `yaml:"output_ebook"`
}

type SeriesConfig struct {
	Name      string         `yaml:"name"`
	Subdir    string         `yaml:"subdir"`
	Title     string         `yaml:"title"`
	Subtitle  string         `yaml:"subtitle"`
	Subseries string         `yaml:"subseries"`
	URL       string         `yaml:"url"`
	Core      string         `yaml:"core"`
	TitleIdx  bool           `yaml:"titleidx"`
	Values    []FieldMapping `yaml:"values"`
	Locations []Location     `yaml:"locations"`
	// Interval is the number of days between issues, used together with Anchor
	// to estimate the next release date. Optional: when Update() successfully
	// fetches metadata for the next unreleased issue from the metadata source,
	// that announced date is stored in state and takes precedence over this estimate.
	Interval  int            `yaml:"interval"`
	Anchor    Anchor         `yaml:"anchor"`
	Types     []string       `yaml:"types"`
	Length    string         `yaml:"length"`
	Complete  bool           `yaml:"complete"`
	ScanFrom  int            `yaml:"scanfrom"`
	Latest    int            `yaml:"latest"`
	States    []string       `yaml:"states"`

	// Derived at load time
	SlugName string `yaml:"-"`
}

type FieldMapping struct {
	Name  string `yaml:"name"`
	Alias string `yaml:"alias"`
}

type Location struct {
	What        string `yaml:"what"`
	ScanPattern string `yaml:"scanpattern"`
}

type Anchor struct {
	Number int    `yaml:"number"`
	Date   string `yaml:"date"`
}

// TemplateData is the data available inside config Go template strings.
type TemplateData struct {
	Number    int
	Title     string
	SubSeries string
	Author    string
}

func LoadMain(path string) (MainConfig, error) {
	var cfg MainConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading main config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing main config: %w", err)
	}
	return cfg, nil
}

func LoadSeries(dir string) ([]SeriesConfig, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("globbing series configs: %w", err)
	}
	var result []SeriesConfig
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var cfg SeriesConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		base := filepath.Base(path)
		cfg.SlugName = strings.TrimSuffix(base, ".yaml")
		result = append(result, cfg)
	}
	return result, nil
}

func LoadAll(configDir string) (MainConfig, []SeriesConfig, error) {
	main, err := LoadMain(filepath.Join(configDir, "main.yaml"))
	if err != nil {
		return MainConfig{}, nil, err
	}
	series, err := LoadSeries(filepath.Join(configDir, "series"))
	if err != nil {
		return MainConfig{}, nil, err
	}
	return main, series, nil
}

// RenderTemplate renders a config template string (subdir, title, etc.) with the given data.
func RenderTemplate(tmpl string, data TemplateData) (string, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template %q: %w", tmpl, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %q: %w", tmpl, err)
	}
	return buf.String(), nil
}
