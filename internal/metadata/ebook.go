package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"time"

	"github.com/hk28/prman/internal/cache"
	"github.com/hk28/prman/internal/config"
)

const opfTemplate = `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="uuid_id">
	<metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
		<dc:identifier opf:scheme="calibre" id="calibre_id">{{.CalibreID}}</dc:identifier>
		<dc:title>{{.Title}}</dc:title>
		<dc:creator opf:role="aut">{{.Author}}</dc:creator>
		<dc:date>{{.Date}}</dc:date>
		<dc:description>{{.Description}}</dc:description>
		<dc:language>de</dc:language>
		<dc:subject>NEW_BOOK</dc:subject>
{{- if .SeriesIndex}}
		<meta content="{{.SeriesIndex}}" name="calibre:series_index"/>
{{- end}}
		<meta content="{{.Timestamp}}" name="calibre:timestamp"/>
{{- if .CoreJSON}}
		<meta content="{{.CoreJSON}}" name="calibre:user_metadata:#core"/>
{{- end}}
	</metadata>
	<guide>
		<reference href="cover.jpg" type="cover" title="Umschlagbild"/>
	</guide>
</package>`

type opfData struct {
	CalibreID   int
	Title       string
	Author      string
	Date        string
	Description string
	SeriesIndex int
	Timestamp   string
	CoreJSON    template.HTML // pre-built JSON blob, HTML-safe
}

// coreMeta mirrors the calibre:user_metadata:#core JSON blob structure.
type coreMeta struct {
	Extra        float64     `json:"#extra#"`
	Datatype     string      `json:"datatype"`
	Name         string      `json:"name"`
	Column       string      `json:"column"`
	CategorySort string      `json:"category_sort"`
	IsCsp        bool        `json:"is_csp"`
	IsCustom     bool        `json:"is_custom"`
	IsEditable   bool        `json:"is_editable"`
	RecIndex     int         `json:"rec_index"`
	LinkColumn   string      `json:"link_column"`
	Display      any `json:"display"`
	IsMultiple   any `json:"is_multiple"`
	IsMultiple2  any `json:"is_multiple2"`
	SearchTerms  []string    `json:"search_terms"`
	IsCategory   bool        `json:"is_category"`
	Table        string      `json:"table"`
	Kind         string      `json:"kind"`
	Value        string      `json:"#value#"`
	Label        string      `json:"label"`
	Colnum       int         `json:"colnum"`
}

func buildCoreMeta(seriesValue string) (string, error) {
	m := coreMeta{
		Extra:        0,
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
	// OPF title format: "<number> - <title>"
	fullTitle := fmt.Sprintf("%d - %s", issue.Number, titleStr)
	if titleStr == "" {
		fullTitle = fmt.Sprintf("%d - ", issue.Number)
	}

	timestamp := time.Now().Format("2006-01-02")
	if issue.ReleaseDate != "" {
		timestamp = issue.ReleaseDate
	}

	d := opfData{
		CalibreID:   calibreID,
		Title:       fullTitle,
		Author:      issue.Author,
		Date:        issue.ReleaseDate,
		Description: issue.Description,
		Timestamp:   timestamp,
	}

	if cfg.TitleIdx {
		d.SeriesIndex = issue.Number
	}

	if cfg.Core != "" {
		coreJSON, err := buildCoreMeta(cfg.Core)
		if err != nil {
			return "", fmt.Errorf("building core meta: %w", err)
		}
		d.CoreJSON = template.HTML(coreJSON)
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
