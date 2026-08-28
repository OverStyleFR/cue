package tui

import (
	"testing"

	"github.com/SuperCoolPencil/cue/internal/domain"
	"github.com/SuperCoolPencil/cue/internal/tui/components"
)

func TestModelPropagateWatchStatus(t *testing.T) {
	m := &Model{
		ColumnStack: NewColumnStack(),
	}

	// Setup stack: Shows -> Seasons -> Episodes
	showCol := components.NewListColumn(components.ColumnTypeShows, "Shows")
	show := &domain.Show{ID: "show1", UnwatchedCount: 10, EpisodeCount: 10}
	showCol.SetItems([]*domain.Show{show})
	m.ColumnStack.Push(showCol, 0)

	seasonCol := components.NewListColumn(components.ColumnTypeSeasons, "Seasons")
	season := &domain.Season{ID: "season1", UnwatchedCount: 5, EpisodeCount: 5}
	seasonCol.SetItems([]*domain.Season{season})
	m.ColumnStack.Push(seasonCol, 0)

	episodeCol := components.NewListColumn(components.ColumnTypeEpisodes, "Episodes")
	episode := &domain.MediaItem{
		ID:       "ep1",
		Type:     domain.MediaTypeEpisode,
		ShowID:   "show1",
		ParentID: "season1",
	}
	episodeCol.SetItems([]*domain.MediaItem{episode})
	m.ColumnStack.Push(episodeCol, 0)

	// Test: Mark episode as watched (watched=true, delta=-1)
	m.propagateWatchStatus(episode, true)

	if show.UnwatchedCount != 9 {
		t.Errorf("expected show unwatched 9, got %d", show.UnwatchedCount)
	}
	if season.UnwatchedCount != 4 {
		t.Errorf("expected season unwatched 4, got %d", season.UnwatchedCount)
	}

	// Test: Mark episode as unwatched (watched=false, delta=+1)
	m.propagateWatchStatus(episode, false)

	if show.UnwatchedCount != 10 {
		t.Errorf("expected show unwatched 10, got %d", show.UnwatchedCount)
	}
	if season.UnwatchedCount != 5 {
		t.Errorf("expected season unwatched 5, got %d", season.UnwatchedCount)
	}
}

func TestUpdateInspectorReloadsPosterWhenRequestKeyChanges(t *testing.T) {
	col := components.NewListColumn(components.ColumnTypeMovies, "Movies")
	col.SetItems([]*domain.MediaItem{{ID: "movie-1", ThumbURL: "https://media/poster-a"}})
	col.SetSize(50, 20)

	m := Model{
		ColumnStack: NewColumnStack(),
		Inspector:   components.NewInspector(),
		MediaClient: &posterClientStub{},
	}
	m.ColumnStack.Push(col, 0)

	if cmd := m.updateInspector(); cmd == nil {
		t.Fatal("expected a poster request for the initial selection")
	}
	firstRequestID := m.posterRequestID

	col.SetItems([]*domain.MediaItem{{ID: "movie-1", ThumbURL: "https://media/poster-b"}})
	m.posterContent = "old poster"
	if cmd := m.updateInspector(); cmd == nil {
		t.Fatal("expected a new poster request after the URL changed")
	}
	if m.posterRequestID == firstRequestID {
		t.Fatal("poster request generation did not advance")
	}
	if m.posterContent != "" {
		t.Fatal("old poster remained visible while the replacement was loading")
	}
}

func TestUpdateInspectorReloadsPosterWhenWidthChanges(t *testing.T) {
	col := components.NewListColumn(components.ColumnTypeMovies, "Movies")
	col.SetItems([]*domain.MediaItem{{ID: "movie-1", ThumbURL: "https://media/poster"}})
	col.SetSize(40, 20)

	m := Model{
		ColumnStack: NewColumnStack(),
		Inspector:   components.NewInspector(),
		MediaClient: &posterClientStub{},
	}
	m.ColumnStack.Push(col, 0)

	if cmd := m.updateInspector(); cmd == nil {
		t.Fatal("expected a poster request for the initial selection")
	}
	firstRequestID := m.posterRequestID

	col.SetSize(70, 20)
	if cmd := m.updateInspector(); cmd == nil {
		t.Fatal("expected a new poster request after the width changed")
	}
	if m.posterRequestID == firstRequestID {
		t.Fatal("poster request generation did not advance after resize")
	}
}

func TestPosterLoadedIgnoresStaleRequest(t *testing.T) {
	m := Model{
		posterRequestID: 2,
		posterItemID:    "movie-1",
		posterContent:   "current poster",
	}

	model, _ := m.Update(PosterLoadedMsg{
		RequestID: 1,
		ItemID:    "movie-1",
		Content:   "stale poster",
	})
	updated := model.(Model)
	if updated.posterContent != "current poster" {
		t.Fatal("stale poster request replaced the current poster")
	}
}

func TestUpdateInspectorClearsPosterWithoutArtwork(t *testing.T) {
	col := components.NewListColumn(components.ColumnTypeShows, "Shows")
	col.SetItems([]*domain.Show{{ID: "show-1"}})

	m := Model{
		ColumnStack:      NewColumnStack(),
		Inspector:        components.NewInspector(),
		posterItemID:     "old-show",
		posterContent:    "old poster",
		posterRequestKey: "old-show\x00old-url\x0020\x000",
	}
	m.ColumnStack.Push(col, 0)

	if cmd := m.updateInspector(); cmd != nil {
		t.Fatal("unexpected poster request for an item without artwork")
	}
	if m.posterItemID != "" || m.posterContent != "" || m.posterRequestKey != "" {
		t.Fatal("old poster state was not cleared")
	}
}
