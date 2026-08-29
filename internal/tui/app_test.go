package tui

import (
	"bytes"
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
	col := components.NewListColumn(components.ColumnTypeShows, "Shows")
	col.SetItems([]*domain.Show{{ID: "show-1", ThumbURL: "https://media/poster-a"}})
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

	col.SetItems([]*domain.Show{{ID: "show-1", ThumbURL: "https://media/poster-b"}})
	m.posterContent = "old poster"
	if cmd := m.updateInspector(); cmd == nil {
		t.Fatal("expected a new poster request after the URL changed")
	}
	if m.posterRequestID == firstRequestID {
		t.Fatal("poster request generation did not advance")
	}
	if m.posterContent != "old poster" {
		t.Fatal("existing poster was cleared while its replacement was loading")
	}
}

func TestUpdateInspectorDoesNotReloadSidebarPosterWhenWidthChanges(t *testing.T) {
	col := components.NewListColumn(components.ColumnTypeShows, "Shows")
	col.SetItems([]*domain.Show{{ID: "show-1", ThumbURL: "https://media/poster"}})
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
	if cmd := m.updateInspector(); cmd != nil {
		t.Fatal("unexpected poster request after a width change")
	}
	if m.posterRequestID != firstRequestID {
		t.Fatal("poster request generation changed after resize")
	}
}

func TestUpdateInspectorDoesNotRequestPosterForEpisodePane(t *testing.T) {
	col := components.NewListColumn(components.ColumnTypeEpisodes, "Episodes")
	col.SetItems([]*domain.MediaItem{{ID: "episode-1", Type: domain.MediaTypeEpisode, ShowThumbURL: "https://media/poster"}})

	m := Model{
		ColumnStack:      NewColumnStack(),
		Inspector:        components.NewInspector(),
		MediaClient:      &posterClientStub{},
		posterItemID:     "show-1",
		posterContent:    "old poster",
		posterRequestKey: "show-1\x00old-url\x0020\x0010",
	}
	m.ColumnStack.Push(col, 0)

	if cmd := m.updateInspector(); cmd != nil {
		t.Fatal("unexpected poster request for episode pane")
	}
	if m.hasPosterState() {
		t.Fatal("episode pane retained a show poster")
	}
}

func TestUpdateInspectorRetainsShowPreviewInEpisodePane(t *testing.T) {
	showCol := components.NewListColumn(components.ColumnTypeShows, "Shows")
	showCol.SetItems([]*domain.Show{{ID: "show-1", ThumbURL: "https://media/show-poster"}})
	episodeCol := components.NewListColumn(components.ColumnTypeEpisodes, "Episodes")
	episodeCol.SetItems([]*domain.MediaItem{{ID: "episode-1", Type: domain.MediaTypeEpisode}})

	m := Model{
		ColumnStack: NewColumnStack(),
		Inspector:   components.NewInspector(),
		MediaClient: &posterClientStub{},
	}
	m.ColumnStack.Push(showCol, 0)
	if cmd := m.updateInspector(); cmd == nil {
		t.Fatal("expected initial show poster request")
	}
	m.posterContent = "show poster"
	m.ColumnStack.Push(episodeCol, 0)

	if cmd := m.updateInspector(); cmd != nil {
		t.Fatal("unexpected poster request after opening episode pane")
	}
	if m.posterItemID != "show-1" || m.posterContent != "show poster" {
		t.Fatal("episode pane did not retain the selected show's preview")
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

func TestPosterFailureClearsRetainedPreviewAfterRequestCompletes(t *testing.T) {
	var output bytes.Buffer
	m := Model{
		posterRequestID:  3,
		posterItemID:     "movie-2",
		posterContent:    "previous poster",
		posterPlacement:  "previous placement",
		posterImageID:    42,
		posterOutput:     &output,
		posterRequestKey: "movie-2\x00url\x0030\x0020",
	}

	model, _ := m.Update(PosterLoadedMsg{RequestID: 3, ItemID: "movie-2"})
	updated := model.(Model)
	if updated.posterContent != "" || updated.posterPlacement != "" || updated.posterImageID != 0 {
		t.Fatal("failed replacement left the previous preview visible")
	}
	if output.Len() == 0 {
		t.Fatal("failed replacement did not delete the previous kitty image")
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

func TestPosterMaxHeightFitsMovieInspector(t *testing.T) {
	movieCol := components.NewListColumn(components.ColumnTypeMovies, "Movies")
	movieCol.SetItems([]*domain.MediaItem{{
		ID:    "movie-1",
		Type:  domain.MediaTypeMovie,
		Title: "Movie",
		// Technical footer is rendered for movies.
		VideoCodec: "H.264",
		AudioCodec: "AAC",
	}})
	movieCol.SetSize(50, 20)

	m := Model{
		ColumnStack: NewColumnStack(),
		Height:      30,
	}
	m.ColumnStack.Push(movieCol, 0)

	maxHeight := m.posterMaxHeight(movieCol)
	contentHeight := m.Height - ChromeHeight
	infoHeight := contentHeight - contentHeight/3
	if maxHeight <= 0 || maxHeight >= infoHeight {
		t.Fatalf("movie poster maxHeight = %d, expected 0 < maxHeight < infoHeight %d", maxHeight, infoHeight)
	}
}

func TestPosterMaxHeightFitsEpisodeInspector(t *testing.T) {
	epCol := components.NewListColumn(components.ColumnTypeEpisodes, "Episodes")
	epCol.SetItems([]*domain.MediaItem{{
		ID:       "ep-1",
		Type:     domain.MediaTypeEpisode,
		Title:    "Episode",
		ParentID: "season-1",
		ShowID:   "show-1",
	}})
	epCol.SetSize(50, 20)

	m := Model{
		ColumnStack: NewColumnStack(),
		Height:      30,
	}
	m.ColumnStack.Push(epCol, 0)

	maxHeight := m.posterMaxHeight(epCol)
	contentHeight := m.Height - ChromeHeight
	listHeight := (55 * contentHeight) / 100
	infoHeight := contentHeight - listHeight
	if maxHeight <= 0 || maxHeight >= infoHeight {
		t.Fatalf("episode poster maxHeight = %d, expected 0 < maxHeight < infoHeight %d", maxHeight, infoHeight)
	}
}
