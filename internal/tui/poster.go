package tui

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
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
func SupportsKittyImage() bool {
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
//
// In kitty mode the image pixels are transmitted once directly to stdout and
// a placement string with Unicode placeholders is returned. Bubble Tea re-emits
// the placement on every render, so the image survives redraws.
func RenderPoster(data []byte, itemID string, widthCells int) string {
	if SupportsKittyImage() {
		pngBytes, imageID, w, h, err := renderKitty(data, itemID, widthCells)
		if err == nil {
			// Transmit the pixels once. The placement (returned below) is
			// re-emitted on each frame, keeping the image visible.
			os.Stdout.WriteString(kittyTransmit(pngBytes, imageID))
			return kittyPlacement(imageID, w, h)
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

// renderKitty decodes and resizes the image for kitty transmission and
// returns the PNG bytes plus the display dimensions in cells.
func renderKitty(data []byte, itemID string, widthCells int) (pngBytes []byte, imageID uint32, wCells, hCells int, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, 0, err
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, 0, 0, 0, fmt.Errorf("invalid image bounds")
	}
	dispW := widthCells * kittyCellWpx
	dispH := (dispW * b.Dy()) / b.Dx()
	if dispH < 1 {
		dispH = 1
	}
	resized := resizeNearest(img, dispW, dispH)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return nil, 0, 0, 0, err
	}

	heightCells := dispH / kittyCellHpx
	if heightCells < 1 {
		heightCells = 1
	}
	return buf.Bytes(), kittyImageID(itemID), widthCells, heightCells, nil
}

// kittyImageID derives a stable numeric image ID for the kitty graphics
// protocol from the item identifier.
func kittyImageID(itemID string) uint32 {
	if itemID == "" {
		return 1
	}
	h := fnv.New32a()
	h.Write([]byte(itemID))
	id := h.Sum32()
	if id == 0 {
		id = 1
	}
	return id
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

// kittyTransmit returns the kitty graphics protocol escape sequence that
// uploads the image pixels to the terminal. It should be written once.
func kittyTransmit(pngBytes []byte, imageID uint32) string {
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
			fmt.Fprintf(&sb, "a=t,f=100,i=%d", imageID)
			first = false
		}
		if last {
			sb.WriteString(",m=0")
		} else {
			sb.WriteString(",m=1")
		}
		sb.WriteString(";")
		sb.WriteString(part)
		sb.WriteString("\x1b\\")
	}
	return sb.String()
}

// kittyPlacement returns the kitty graphics protocol escape sequence that
// places an already-transmitted image at the current cursor position.
func kittyPlacement(imageID uint32, widthCells, heightCells int) string {
	var sb strings.Builder
	sb.WriteString("\x1b_G")
	fmt.Fprintf(&sb, "a=p,i=%d,c=%d,r=%d,C=1", imageID, widthCells, heightCells)
	sb.WriteString("\x1b\\")
	return sb.String()
}

// kittyPlaceholders returns just the Unicode placeholder characters that fill
// the cells of a kitty image placement. Bubble Tea re-emits these on every
// render so the image survives redraws without re-uploading pixels.
func kittyPlaceholders(widthCells, heightCells int) string {
	var sb strings.Builder
	const placeholder = "\U0010EEEE"
	for i := 0; i < heightCells; i++ {
		sb.WriteString(strings.Repeat(placeholder, widthCells))
		if i < heightCells-1 {
			sb.WriteString("\n")
		}
	}
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

		if SupportsKittyImage() {
			pngBytes, imageID, w, h, err := renderKitty(data, itemID, widthCells)
			if err == nil {
				// Transmit the pixels once. The placement + placeholders are
				// rendered in the View; on subsequent frames only placeholders
				// are re-emitted to keep the image visible without re-upload.
				os.Stdout.WriteString(kittyTransmit(pngBytes, imageID))
				return PosterLoadedMsg{
					ItemID:      itemID,
					Content:     kittyPlaceholders(w, h),
					Placement:   kittyPlacement(imageID, w, h),
					ImageID:     imageID,
					WidthCells:  w,
					HeightCells: h,
				}
			}
		}

		content := RenderPoster(data, itemID, widthCells)
		if content == "" {
			return nil
		}
		return PosterLoadedMsg{ItemID: itemID, Content: content}
	}
}
