package tui_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

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
	out := tui.RenderPoster(makePosterPNG(t), 40)
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
	out := tui.RenderPoster(makePosterPNG(t), 40)
	if !bytes.HasPrefix([]byte(out), []byte("\x1b_G")) {
		t.Fatalf("kitty poster missing escape prefix")
	}
}
