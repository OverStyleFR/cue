package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/SuperCoolPencil/cue/internal/domain"
	"github.com/SuperCoolPencil/cue/internal/tui/components"
	tea "github.com/charmbracelet/bubbletea"
)

func TestMoviePosterAppearsInView(t *testing.T) {
	os.Unsetenv("KITTY_WINDOW_ID")
	os.Unsetenv("TERM")
	os.Unsetenv("ZELLIJ")
	os.Unsetenv("ZELLIJ_SESSION_NAME")
	os.Unsetenv("TMUX")
	os.Unsetenv("STY")
	if SupportsKittyImage() {
		t.Skip("kitty env detected; testing ASCII path only")
	}

	movieCol := components.NewListColumn(components.ColumnTypeMovies, "Movies")
	movieCol.SetItems([]*domain.MediaItem{{
		ID:       "movie-1",
		Type:     domain.MediaTypeMovie,
		Title:    "Test Movie",
		ThumbURL: "https://server/poster.jpg",
	}})
	movieCol.SetSize(50, 20)

	m := Model{
		ColumnStack: NewColumnStack(),
		Inspector:   components.NewInspector(),
		Width:       120,
		Height:      40,
	}
	m.ColumnStack.Push(components.NewLibraryColumn([]domain.Library{{ID: "lib-1", Name: "Movies", Type: "movie"}}), 0)
	m.ColumnStack.Push(movieCol, 0)

	// Simulate terminal size initialization.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(Model)

	// Simulate a poster loaded asynchronously.
	m.posterRequestID = 1
	m.posterItemID = "movie-1"
	m.posterContent = "ASCII_POSTER_CONTENT"
	m.posterRequestKey = "movie-1\x00https://server/poster.jpg\x0047\x0012"

	view := m.View()
	if !strings.Contains(view, "ASCII_POSTER_CONTENT") {
		t.Fatalf("movie poster not rendered in view; got:\n%s", view)
	}
}
