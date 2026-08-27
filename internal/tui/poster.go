package tui

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"image/png"
	"os"
	"strings"

	"github.com/SuperCoolPencil/cue/internal/domain"
	"github.com/SuperCoolPencil/cue/internal/mediaserver"
	gopixels "github.com/saran13raj/go-pixels"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	// posterMaxWidth caps the rendered poster width in cells.
	posterMaxWidth = 44
	posterMinWidth = 20

	// kitty cell pixel estimates used to size transmitted images.
	kittyCellWpx = 8
	kittyCellHpx = 16
)

// SupportsKittyImage reports whether the terminal likely supports the kitty
// graphics protocol (used to draw real poster images instead of ASCII art).
//
// The kitty graphics protocol is disabled by default: in a scrolling full-screen
// TUI the image must be re-emitted or use Unicode placeholders on every frame
// to survive Bubble Tea redraws. ASCII art is reliable everywhere. Users can
// opt into the experimental kitty protocol with CUE_KITTY_IMAGES=1.
func SupportsKittyImage() bool {
	if os.Getenv("CUE_KITTY_IMAGES") != "1" {
		return false
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if strings.Contains(os.Getenv("TERM"), "kitty") {
		return true
	}
	return false
}

// PosterURL returns the best poster/artwork URL for a selected list item.
// For episodes it prefers the parent series poster (ShowThumbURL).
func PosterURL(item interface{}) string {
	switch v := item.(type) {
	case *domain.MediaItem:
		if v.ShowThumbURL != "" {
			return v.ShowThumbURL
		}
		if v.ThumbURL != "" {
			return v.ThumbURL
		}
		return v.ArtURL
	case *domain.Show:
		return v.ThumbURL
	case *domain.Season:
		return v.ThumbURL
	}
	return ""
}

// RenderPoster converts raw image bytes into a terminal-renderable string,
// preferring the kitty image protocol when supported, otherwise ASCII art.
func RenderPoster(data []byte, widthCells int) string {
	if SupportsKittyImage() {
		if s, err := renderKitty(data, widthCells); err == nil {
			return s
		}
	}
	s, err := renderASCII(data, widthCells)
	if err != nil {
		return ""
	}
	return s
}

func renderASCII(data []byte, widthCells int) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	// "halfcell" mode doubles vertical resolution using block characters.
	return gopixels.FromImageStream(img, widthCells, 0, "halfcell", true)
}

func renderKitty(data []byte, widthCells int) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return "", fmt.Errorf("invalid image bounds")
	}
	dispW := widthCells * kittyCellWpx
	dispH := (dispW * b.Dy()) / b.Dx()
	if dispH < 1 {
		dispH = 1
	}
	resized := resizeNearest(img, dispW, dispH)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return "", err
	}

	heightCells := dispH / kittyCellHpx
	if heightCells < 1 {
		heightCells = 1
	}
	return kittyEscape(buf.Bytes(), dispW, dispH, heightCells), nil
}

// resizeNearest downscales an image to the given pixel dimensions using a
// simple nearest-neighbor sampling (no external dependency).
func resizeNearest(img image.Image, w, h int) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return dst
	}
	for y := 0; y < h; y++ {
		sy := b.Min.Y + y*b.Dy()/h
		for x := 0; x < w; x++ {
			sx := b.Min.X + x*b.Dx()/w
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	return dst
}

// kittyEscape builds a kitty graphics protocol transmission placed at the
// cursor, followed by blank lines reserving vertical space.
func kittyEscape(pngBytes []byte, dispW, dispH, heightCells int) string {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	const chunk = 4096
	var sb strings.Builder
	first := true
	remaining := b64
	for len(remaining) > 0 {
		n := len(remaining)
		if n > chunk {
			n = chunk
		}
		part := remaining[:n]
		remaining = remaining[n:]
		last := len(remaining) == 0

		sb.WriteString("\x1b_G")
		if first {
			fmt.Fprintf(&sb, "f=32,a=T,s=%d,v=%d", dispW, dispH)
			first = false
		}
		if last {
			sb.WriteString(",c=1,m=0")
		} else {
			sb.WriteString(",m=1")
		}
		sb.WriteString(";")
		sb.WriteString(part)
		sb.WriteString("\x1b\\")
	}
	// Move the cursor below the image so subsequent text doesn't overlap it.
	sb.WriteString(strings.Repeat("\n", heightCells))
	return sb.String()
}

// FetchPosterCmd returns a command that downloads and renders the poster for
// the given item, emitting a PosterLoadedMsg on success.
func FetchPosterCmd(client mediaserver.MediaSource, itemID, url string, widthCells int) tea.Cmd {
	if url == "" {
		return nil
	}
	return func() tea.Msg {
		data, err := client.GetImage(context.Background(), url)
		if err != nil {
			return nil
		}
		content := RenderPoster(data, widthCells)
		if content == "" {
			return nil
		}
		return PosterLoadedMsg{ItemID: itemID, Content: content}
	}
}
