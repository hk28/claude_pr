package metadata

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hk28/prman/internal/cache"
	"github.com/hk28/prman/internal/config"
)

type AudioMetadata struct {
	Title         string   `json:"title"`
	Subtitle      string   `json:"subtitle"`
	Authors       []string `json:"authors"`
	Series        []string `json:"series"`
	PublishedYear string   `json:"publishedYear"`
	Description   string   `json:"description"`
	Language      string   `json:"language"`
}

// GenerateAudio produces the metadata.json content for an audiobook issue.
func GenerateAudio(cfg config.SeriesConfig, issue cache.ScrapedIssue) (AudioMetadata, error) {
	tdata := config.TemplateData{
		Number:      issue.Number,
		Title:       issue.Title,
		SubSeries:   issue.SubSeries,
		Author:      issue.Author,
		Description: issue.Description,
	}

	title, err := config.RenderTemplate(cfg.Title, tdata)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("rendering title: %w", err)
	}

	subtitle, err := config.RenderTemplate(cfg.Subtitle, tdata)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("rendering subtitle: %w", err)
	}

	subseries, err := config.RenderTemplate(cfg.Subseries, tdata)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("rendering subseries: %w", err)
	}

	// subseries is comma-separated → split into array
	var seriesArr []string
	for _, s := range strings.Split(subseries, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			seriesArr = append(seriesArr, s)
		}
	}

	// Authors: split on " und " or ", "
	authors := splitAuthors(issue.Author)

	// Description: HTML link to source page
	description := buildDescription(issue.SourceURL)

	return AudioMetadata{
		Title:         title,
		Subtitle:      subtitle,
		Authors:       authors,
		Series:        seriesArr,
		PublishedYear: issue.ReleaseDate,
		Description:   description,
		Language:      "de",
	}, nil
}

// MarshalAudio returns the indented JSON bytes for audio metadata.
func MarshalAudio(m AudioMetadata) ([]byte, error) {
	return json.MarshalIndent(m, "", "\t")
}

func splitAuthors(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	// Split on " und " first
	parts := strings.Split(s, " und ")
	var result []string
	for _, p := range parts {
		// Also split on ", "
		for _, q := range strings.Split(p, ", ") {
			q = strings.TrimSpace(q)
			if q != "" {
				result = append(result, q)
			}
		}
	}
	return result
}

func buildDescription(sourceURL string) string {
	if sourceURL == "" {
		return "<p></p>"
	}
	return fmt.Sprintf(`<p></p><br><p><a href="%s" target="_blank" rel="noopener noreferrer" >%s</a></p>`, sourceURL, sourceURL)
}
