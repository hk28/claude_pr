package metadata

import (
	"strings"
	"testing"

	"github.com/hk28/prman/internal/cache"
	"github.com/hk28/prman/internal/config"
)

func TestGenerateOPF2757(t *testing.T) {
	cfg := config.SeriesConfig{
		Name:     "Pr.Heft",
		Core:     "PR.Heft",
		Title:    "{{.Title}}",
		TitleIdx: true,
	}
	issue := cache.ScrapedIssue{
		Number:      2757,
		Title:       "Das Sorgenkind",
		Author:      "Tanja Kinkel",
		Description: "Eine Jugend auf dem Planeten Gloster– ein Leben in Demütigung und Gefahr",
		ReleaseDate: "2014-06-01T23:00:00+00:00",
		SubSeries:   "2700 Das Atopische Tribunal",
	}

	opf, err := GenerateOPF(cfg, issue, 10914)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("generated OPF:\n%s", opf)

	checks := []string{
		"<dc:title>2757 - Das Sorgenkind</dc:title>",
		"<dc:creator opf:file-as=\"Kinkel, Tanja\" opf:role=\"aut\">Tanja Kinkel</dc:creator>",
		"<dc:date>2014-06-01T23:00:00", // +00:00 might be escaped, so check prefix
		"<dc:language>deu</dc:language>",
		"<dc:subject>PR Heft</dc:subject>",
		"<meta name=\"calibre:series_index\" content=\"2757\"/>",
		"calibre:user_metadata:#core",
	}
	for _, check := range checks {
		if strings.HasPrefix(check, "<dc:date>") {
			if !strings.Contains(opf, check) {
				t.Errorf("OPF output missing expected content prefix: %s", check)
			}
			continue
		}
		if !strings.Contains(opf, check) {
			t.Errorf("OPF output missing expected content: %s", check)
		}
	}
}
