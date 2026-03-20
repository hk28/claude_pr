package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/hk28/prman/internal/cache"
	"github.com/hk28/prman/internal/config"
)

const opfTemplate = `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="uuid_id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
<dc:identifier opf:scheme="calibre" id="calibre_id">{{.CalibreID}}</dc:identifier>
<dc:identifier opf:scheme="uuid" id="uuid_id">{{.UUID}}</dc:identifier>
<dc:title>{{.Title}}</dc:title>
<dc:creator opf:file-as="{{.AuthorFileAs}}" opf:role="aut">{{.Author}}</dc:creator>
<dc:contributor opf:file-as="calibre" opf:role="bkp">calibre (6.2.1) [https://calibre-ebook.com]</dc:contributor>
<dc:date>{{.Date}}</dc:date>
<dc:description>{{.Description}}</dc:description>
<dc:publisher>{{.Publisher}}</dc:publisher>
<dc:language>{{.Language}}</dc:language>
<dc:subject>{{.Subject}}</dc:subject>
<meta name="calibre:author_link_map" content="{{.AuthorLinkJSON}}"/>
<meta name="calibre:series" content="{{.Series}}"/>
<meta name="calibre:series_index" content="{{.SeriesIndex}}"/>
<meta name="calibre:timestamp" content="{{.Timestamp}}"/>
<meta name="calibre:title_sort" content="{{.TitleSort}}"/>
<meta name="calibre:user_metadata:#core" content="{{.CoreJSON}}"/>
<meta name="calibre:user_metadata:#user_categories" content="{{.UserCategoriesJSON}}"/>
</metadata>
<guide>
<reference href="cover.jpg" type="cover" title="Umschlagbild"/>
</guide>
</package>`

type opfData struct {
	CalibreID          int
	UUID               string
	Title              string
	Author             string
	AuthorFileAs       string
	Date               string
	Description        string
	Publisher          string
	Language           string
	Subject            string
	Series             string
	SeriesIndex        int
	Timestamp          string
	TitleSort          string
	CoreJSON           template.HTML
	AuthorLinkJSON     template.HTML
	UserCategoriesJSON template.HTML
}

// coreMeta mirrors the calibre:user_metadata:#core JSON blob structure.
type coreMeta struct {
	Extra        float64  `json:"#extra#"`
	Datatype     string   `json:"datatype"`
	Name         string   `json:"name"`
	Column       string   `json:"column"`
	CategorySort string   `json:"category_sort"`
	IsCsp        bool     `json:"is_csp"`
	IsCustom     bool     `json:"is_custom"`
	IsEditable   bool     `json:"is_editable"`
	RecIndex     int      `json:"rec_index"`
	LinkColumn   string   `json:"link_column"`
	Display      any      `json:"display"`
	IsMultiple   any      `json:"is_multiple"`
	IsMultiple2  any      `json:"is_multiple2"`
	SearchTerms  []string `json:"search_terms"`
	IsCategory   bool     `json:"is_category"`
	Table        string   `json:"table"`
	Kind         string   `json:"kind"`
	Value        string   `json:"#value#"`
	Label        string   `json:"label"`
	Colnum       int      `json:"colnum"`
}

func buildCoreMeta(seriesValue string, issueNumber int) (string, error) {
	m := coreMeta{
		Extra:        float64(issueNumber),
		Datatype:     "series",
		Name:         "Core",
		Column:       "value",
		CategorySort: "value",
		IsCsp:        false,
		IsCustom:     true,
		IsEditable:   true,
		RecIndex:     24,
		LinkColumn:   "value",
		Display:      map[string]any{},
		IsMultiple:   nil,
		IsMultiple2:  map[string]any{},
		SearchTerms:  []string{"#core"},
		IsCategory:   true,
		Table:        "custom_column_1",
		Kind:         "field",
		Value:        seriesValue,
		Label:        "core",
		Colnum:       1,
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func buildUUID(issue cache.ScrapedIssue) string {
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", issue.Number)
}

func buildAuthorFileAs(author string) string {
	author = strings.TrimSpace(author)
	if author == "" {
		return ""
	}
	if strings.Contains(author, ",") {
		parts := strings.Split(author, ",")
		return strings.TrimSpace(parts[0]) + ", " + strings.TrimSpace(parts[1])
	}
	parts := strings.Fields(author)
	if len(parts) < 2 {
		return author
	}
	return fmt.Sprintf("%s, %s", parts[len(parts)-1], strings.Join(parts[:len(parts)-1], " "))
}

func buildAuthorLinkJSON(author string) (string, error) {
	if strings.TrimSpace(author) == "" {
		return "{}", nil
	}
	m := map[string]string{author: ""}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func buildSeriesMeta(cfg config.SeriesConfig, issue cache.ScrapedIssue) string {
	series := cfg.Core
	if series == "" {
		series = cfg.Name
	}
	if issue.SubSeries != "" {
		if !strings.Contains(issue.SubSeries, series) {
			series = fmt.Sprintf("%s.%s", series, issue.SubSeries)
		}
	}
	return series
}

func buildTitleSort(cfg config.SeriesConfig, issue cache.ScrapedIssue) string {
	if cfg.Name != "" {
		return cfg.Name
	}
	return fmt.Sprintf("%d - %s", issue.Number, issue.Title)
}

func buildUserCategoriesJSON() (string, error) {
	m := map[string][]string{
		"Fantasy Modern":  {},
		"Genre":           {},
		"Lange Serien":    {},
		"Meine Serien":    {},
		"PR Aktuell":      {},
		"SciFi Deutsch":   {},
		"SciFi Klassiker": {},
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GenerateOPF produces the .opf XML content for an ebook issue.
func GenerateOPF(cfg config.SeriesConfig, issue cache.ScrapedIssue, calibreID int) (string, error) {
	tdata := config.TemplateData{
		Number:    issue.Number,
		Title:     issue.Title,
		SubSeries: issue.SubSeries,
		Author:    issue.Author,
	}

	titleStr, err := config.RenderTemplate(cfg.Title, tdata)
	if err != nil {
		return "", fmt.Errorf("rendering title: %w", err)
	}
	fullTitle := fmt.Sprintf("%d - %s", issue.Number, titleStr)
	if titleStr == "" {
		fullTitle = fmt.Sprintf("%d - ", issue.Number)
	}

	timestamp := time.Now().Format("2006-01-02")
	if issue.ReleaseDate != "" {
		timestamp = issue.ReleaseDate
	}

	if issue.ReleaseDate == "" {
		issue.ReleaseDate = timestamp
	}

	coreJSON, err := buildCoreMeta(cfg.Core, issue.Number)
	if err != nil {
		return "", fmt.Errorf("building core meta: %w", err)
	}

	authorLinkJSON, err := buildAuthorLinkJSON(issue.Author)
	if err != nil {
		return "", fmt.Errorf("building author link map: %w", err)
	}

	userCategoriesJSON, err := buildUserCategoriesJSON()
	if err != nil {
		return "", fmt.Errorf("building user categories: %w", err)
	}

	d := opfData{
		CalibreID:          calibreID,
		UUID:               buildUUID(issue),
		Title:              fullTitle,
		Author:             issue.Author,
		AuthorFileAs:       buildAuthorFileAs(issue.Author),
		Date:               issue.ReleaseDate,
		Description:        issue.Description,
		Publisher:          "Perry Rhodan digital",
		Language:           "deu",
		Subject:            strings.ReplaceAll(cfg.Core, ".", " "),
		Series:             buildSeriesMeta(cfg, issue),
		SeriesIndex:        issue.Number,
		Timestamp:          timestamp,
		TitleSort:          buildTitleSort(cfg, issue),
		CoreJSON:           template.HTML(coreJSON),
		AuthorLinkJSON:     template.HTML(authorLinkJSON),
		UserCategoriesJSON: template.HTML(userCategoriesJSON),
	}

	if cfg.TitleIdx {
		d.SeriesIndex = issue.Number
	}

	tmpl, err := template.New("opf").Parse(opfTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}
