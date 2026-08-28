package tui_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/SuperCoolPencil/cue/internal/domain"
	"github.com/SuperCoolPencil/cue/internal/tui"
)

func makePosterPNG(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 80, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 80; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 3), uint8(y * 2), 150, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRenderPosterASCII(t *testing.T) {
	os.Unsetenv("KITTY_WINDOW_ID")
	os.Unsetenv("TERM")
	if tui.SupportsKittyImage() {
		t.Skip("kitty env detected; testing ASCII path only")
	}
	out := tui.RenderPoster(makePosterPNG(t), "test", 40)
	if len(out) < 100 {
		t.Fatalf("ASCII poster too short: %d", len(out))
	}
	if !bytes.ContainsAny([]byte(out), "▀▄█▌▐░▒▓") {
		t.Fatalf("no block chars in ASCII poster")
	}
}

func TestRenderPosterKitty(t *testing.T) {
	os.Setenv("TERM", "xterm-kitty")
	defer os.Unsetenv("TERM")
	if !tui.SupportsKittyImage() {
		t.Skip("kitty not detected")
	}
	out := tui.RenderPoster(makePosterPNG(t), "test", 40)
	if !bytes.Contains([]byte(out), []byte("a=p,U=1")) {
		t.Fatalf("kitty poster missing virtual placement")
	}
	if !bytes.Contains([]byte(out), []byte("\x1b[38;2;")) {
		t.Fatalf("kitty poster missing image ID color")
	}
	if !bytes.Contains([]byte(out), []byte("\U0010EEEE")) {
		t.Fatalf("kitty poster missing placeholders")
	}
	if !bytes.Contains([]byte(out), []byte("\u0305")) {
		t.Fatalf("kitty poster missing row/column diacritics")
	}
}

func TestPosterURLPrefersMoviePosterOverSeriesPoster(t *testing.T) {
	movie := &domain.MediaItem{
		Type:         domain.MediaTypeMovie,
		ThumbURL:     "movie-poster",
		ShowThumbURL: "series-poster",
	}
	if got := tui.PosterURL(movie); got != "movie-poster" {
		t.Fatalf("movie poster URL = %q", got)
	}
}
