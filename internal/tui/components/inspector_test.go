package components

import (
	"strings"
	"testing"

	"github.com/SuperCoolPencil/cue/internal/domain"
)

func TestInspectorRendering(t *testing.T) {
	i := NewInspector()
	i.SetSize(100, 20)

	item := &domain.MediaItem{
		Title:   "Test Movie",
		AirDate: "2023-10-25",
		Year:    2023,
		Type:    domain.MediaTypeMovie,
	}

	i.SetItem(item)
	view := i.View()

	if !strings.Contains(view, "2023-10-25") {
		t.Errorf("view does not contain air date; got:\n%s", view)
	}

	// Test fallback to year
	itemNoAirDate := &domain.MediaItem{
		Title: "Test Movie 2",
		Year:  2022,
		Type:  domain.MediaTypeMovie,
	}
	i.SetItem(itemNoAirDate)
	view2 := i.View()
	if !strings.Contains(view2, "2022") {
		t.Errorf("view does not contain year fallback; got:\n%s", view2)
	}
}

func TestInspectorEpisodeRendering(t *testing.T) {
	i := NewInspector()
	i.SetSize(100, 20)

	item := &domain.MediaItem{
		Title:      "Test Episode",
		AirDate:    "2023-11-01",
		SeasonNum:  1,
		EpisodeNum: 5,
		Type:       domain.MediaTypeEpisode,
	}

	i.SetItem(item)
	view := i.View()

	if !strings.Contains(view, "S01E05") {
		t.Errorf("view does not contain episode code; got:\n%s", view)
	}
	if !strings.Contains(view, "2023-11-01") {
		t.Errorf("view does not contain air date; got:\n%s", view)
	}
}

func TestInspectorTruncatesOversizedHeader(t *testing.T) {
	i := NewInspector()
	i.SetSize(40, 12)

	// Create a poster that would naturally occupy far more lines than the panel.
	bigPoster := strings.Repeat("poster line\n", 20)
	bigPoster = strings.TrimSuffix(bigPoster, "\n")

	i.SetItem(&domain.MediaItem{ID: "movie-1", Type: domain.MediaTypeMovie, Title: "Movie"})
	i.SetPoster(bigPoster)

	view := i.View()
	lines := strings.Split(view, "\n")
	// Interior height = outer height (12) - border (2) = 10 lines.
	if len(lines) > 12 {
		t.Fatalf("inspector view has %d lines, expected at most 12; got:\n%s", len(lines), view)
	}
	// The panel should still render the title and at least some of the poster.
	if !strings.Contains(view, "Info") {
		t.Fatalf("inspector view missing title; got:\n%s", view)
	}
}

func TestInspectorPlacesPosterBesideMetadata(t *testing.T) {
	i := NewInspector()
	i.SetSize(60, 14)
	i.SetItem(&domain.MediaItem{ID: "movie-1", Type: domain.MediaTypeMovie, Title: "Side-by-side metadata"})
	i.SetPoster("POSTER\nPOSTER")

	view := i.View()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "POSTER") && strings.Contains(line, "Side-by-side") {
			return
		}
	}
	t.Fatalf("poster and metadata were not rendered side-by-side; got:\n%s", view)
}
