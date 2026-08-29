package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/SuperCoolPencil/cue/internal/domain"
	"github.com/SuperCoolPencil/cue/internal/tui/components"
	tea "github.com/charmbracelet/bubbletea"
)

func TestShowPosterAppearsInView(t *testing.T) {
	_ = os.Unsetenv("KITTY_WINDOW_ID")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("ZELLIJ")
	_ = os.Unsetenv("ZELLIJ_SESSION_NAME")
	_ = os.Unsetenv("TMUX")
	_ = os.Unsetenv("STY")
	if SupportsKittyImage() {
		t.Skip("kitty env detected; testing ASCII path only")
	}

	showCol := components.NewListColumn(components.ColumnTypeShows, "Shows")
	showCol.SetItems([]*domain.Show{{
		ID:       "show-1",
		Title:    "Test Show",
		ThumbURL: "https://server/poster.jpg",
	}})
	showCol.SetSize(50, 20)

	m := Model{
		ColumnStack: NewColumnStack(),
		Inspector:   components.NewInspector(),
		Width:       120,
		Height:      40,
	}
	m.ColumnStack.Push(components.NewLibraryColumn([]domain.Library{{ID: "lib-1", Name: "Shows", Type: "show"}}), 0)
	m.ColumnStack.Push(showCol, 0)

	// Simulate terminal size initialization.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(Model)
	viewWithoutPoster := m.View()

	// Simulate a poster loaded asynchronously.
	m.posterRequestID = 1
	m.posterItemID = "show-1"
	m.posterContent = "ASCII_POSTER_CONTENT"
	m.posterPlacement = kittyPlacement(1, 20, 1)
	m.posterRequestKey = "show-1\x00https://server/poster.jpg\x0020\x0012"

	view := m.View()
	if !strings.Contains(view, "ASCII_POSTER_CONTENT") {
		t.Fatalf("show poster not rendered in view; got:\n%s", view)
	}
	if got, want := strings.Count(view, "\n"), strings.Count(viewWithoutPoster, "\n"); got != want {
		t.Fatalf("poster changed view height: got %d lines, want %d", got+1, want+1)
	}
}
