package tui_test

import (
	"bytes"
	"encoding/base64"
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

// clearPosterEnv removes environment variables that influence poster renderer
// detection and returns their previous values so tests can restore them.
func clearPosterEnv(t *testing.T) map[string]string {
	t.Helper()
	keys := []string{"KITTY_WINDOW_ID", "TERM", "ZELLIJ", "ZELLIJ_SESSION_NAME", "TMUX", "STY"}
	prev := make(map[string]string, len(keys))
	for _, k := range keys {
		prev[k] = os.Getenv(k)
		_ = os.Unsetenv(k)
	}
	return prev
}

// restorePosterEnv restores environment variables saved by clearPosterEnv.
func restorePosterEnv(t *testing.T, prev map[string]string) {
	t.Helper()
	for k, v := range prev {
		if v == "" {
			_ = os.Unsetenv(k)
		} else {
			_ = os.Setenv(k, v)
		}
	}
}

func TestRenderPosterASCII(t *testing.T) {
	prev := clearPosterEnv(t)
	defer restorePosterEnv(t, prev)
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
	prev := clearPosterEnv(t)
	defer restorePosterEnv(t, prev)
	_ = os.Setenv("TERM", "xterm-kitty")
	defer func() { _ = os.Unsetenv("TERM") }()
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

func TestSupportsKittyImageDetectsZellij(t *testing.T) {
	prev := clearPosterEnv(t)
	defer restorePosterEnv(t, prev)
	_ = os.Setenv("TERM", "xterm-kitty")
	_ = os.Setenv("ZELLIJ_SESSION_NAME", "test-session")
	if tui.SupportsKittyImage() {
		t.Fatal("kitty should be disabled inside Zellij")
	}
}

func TestSupportsKittyImageDetectsTmux(t *testing.T) {
	prev := clearPosterEnv(t)
	defer restorePosterEnv(t, prev)
	_ = os.Setenv("KITTY_WINDOW_ID", "1")
	_ = os.Setenv("TMUX", "/tmp/tmux-1000/default")
	if tui.SupportsKittyImage() {
		t.Fatal("kitty should be disabled inside tmux")
	}
}

func TestSupportsKittyImageDetectsScreen(t *testing.T) {
	prev := clearPosterEnv(t)
	defer restorePosterEnv(t, prev)
	_ = os.Setenv("TERM", "xterm-kitty")
	_ = os.Setenv("STY", "12345.pts-0")
	if tui.SupportsKittyImage() {
		t.Fatal("kitty should be disabled inside screen")
	}
}

func TestSupportsKittyImageEnabledNatively(t *testing.T) {
	prev := clearPosterEnv(t)
	defer restorePosterEnv(t, prev)
	_ = os.Setenv("KITTY_WINDOW_ID", "1")
	if !tui.SupportsKittyImage() {
		t.Fatal("kitty should be enabled outside multiplexers")
	}
}

// tinyWebP is a minimal 80x120 WebP image generated for regression testing.
const tinyWebP = "UklGRgYBAABXRUJQVlA4IPoAAABwBwCdASpQAHgAPpFGoUwlo6MiInVYOLASCWkAVH7i0GZrmAA5IpAFuVuZIbxlSS5YGJwEtsKfqP4Dnx/RuVuZLpyIAAD+8PGr9W0wdVn/160tzdMAADh7q8ob2n/uyWKly/j6URpGgPdOD7819/0ScdNcsP+u8cWUN7FCeCr5zHq9/ngygqXg7wL1U580gy2mo8YE7dsW/Z05/b7l3GR4apGnuFH0uHykvcjmzAQXWjTwsgvjR37IhW+hWSGPvu183CkK77b5jDK4IQflq1X/2V+TPfMyKekajtHpwf/2woViYRzJJsGvJ8WLo+gR6d0FOjT5tygAAAAA"

func TestRenderPosterDecodesWebP(t *testing.T) {
	prev := clearPosterEnv(t)
	defer restorePosterEnv(t, prev)
	data, err := base64.StdEncoding.DecodeString(tinyWebP)
	if err != nil {
		t.Fatal(err)
	}
	out := tui.RenderPoster(data, "webp-test", 40)
	if len(out) < 50 {
		t.Fatalf("WebP poster not rendered, output length: %d", len(out))
	}
	if !bytes.ContainsAny([]byte(out), "▀▄█▌▐░▒▓") {
		t.Fatalf("no block chars in WebP ASCII poster")
	}
}
