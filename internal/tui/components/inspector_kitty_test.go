package components

import (
	"strings"
	"testing"

	"github.com/SuperCoolPencil/cue/internal/domain"
)

func TestInspectorPreservesKittyPoster(t *testing.T) {
	insp := NewInspector()
	insp.SetSize(40, 30)
	insp.SetItem(&domain.MediaItem{ID: "episode-1", Type: domain.MediaTypeEpisode, Title: "Episode"})
	poster := "\x1b_Ga=p,U=1,i=42,c=2,r=2,q=2\x1b\\\x1b[38;2;0;0;42m\U0010EEEE\u0305\u0305\U0010EEEE\u0305\u030D\n\U0010EEEE\u030D\u0305\U0010EEEE\u030D\u030D\x1b[39m"
	insp.SetPoster(poster)

	out := insp.View()
	if !strings.Contains(out, "a=p,U=1") {
		t.Fatal("inspector removed kitty placement escape")
	}
	if !strings.Contains(out, "\U0010EEEE") {
		t.Fatal("inspector removed kitty placeholders")
	}
}
