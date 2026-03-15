package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ScrapedIssue is cached on disk at data/cache/<slug>/<number>.json.
type ScrapedIssue struct {
	Number      int               `json:"number"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	ReleaseDate string            `json:"releaseDate"` // "YYYY-MM-DD"
	SubSeries   string            `json:"subSeries"`
	SourceURL   string            `json:"sourceURL"` // final URL after redirect
	CachedAt    time.Time         `json:"cachedAt"`
	Extra       map[string]string `json:"extra,omitempty"`
}

type Cache struct {
	BaseDir string
}

func New(baseDir string) *Cache {
	return &Cache{BaseDir: baseDir}
}

func (c *Cache) path(slug string, number int) string {
	return filepath.Join(c.BaseDir, slug, strconv.Itoa(number)+".json")
}

func (c *Cache) Exists(slug string, number int) bool {
	_, err := os.Stat(c.path(slug, number))
	return err == nil
}

func (c *Cache) Get(slug string, number int) (ScrapedIssue, bool) {
	data, err := os.ReadFile(c.path(slug, number))
	if err != nil {
		return ScrapedIssue{}, false
	}
	var issue ScrapedIssue
	if err := json.Unmarshal(data, &issue); err != nil {
		return ScrapedIssue{}, false
	}
	return issue, true
}

func (c *Cache) Set(slug string, issue ScrapedIssue) error {
	dir := filepath.Join(c.BaseDir, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	data, err := json.MarshalIndent(issue, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path(slug, issue.Number), data, 0644)
}

func (c *Cache) List(slug string) ([]int, error) {
	dir := filepath.Join(c.BaseDir, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var nums []int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		nums = append(nums, n)
	}
	return nums, nil
}
