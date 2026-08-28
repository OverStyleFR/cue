package tui

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	_ "golang.org/x/image/webp"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/SuperCoolPencil/cue/internal/domain"
	"github.com/SuperCoolPencil/cue/internal/mediaserver"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi/kitty"
	gopixels "github.com/saran13raj/go-pixels"
)

const (
	// posterMaxWidth caps the rendered poster width in cells.
	posterMaxWidth      = 44
	posterMinWidth      = 20
	posterFetchAttempts = 3

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
		if v.Type == domain.MediaTypeEpisode && v.ShowThumbURL != "" {
			return v.ShowThumbURL
		}
		if v.ThumbURL != "" {
			return v.ThumbURL
		}
		if v.ShowThumbURL != "" {
			return v.ShowThumbURL
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
		pngBytes, imageID, w, h, err := renderKitty(data, itemID, 0, widthCells, 0)
		if err == nil {
			// Transmit the pixels once. The placement (returned below) is
			// re-emitted on each frame, keeping the image visible.
			os.Stdout.WriteString(kittyTransmit(pngBytes, imageID))
			return kittyPlacement(imageID, w, h) + kittyPlaceholders(imageID, w, h)
		}
	}
	s, err := renderASCII(data, widthCells, 0)
	if err != nil {
		return ""
	}
	return s
}

func renderASCII(data []byte, widthCells, maxHeightCells int) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	// "halfcell" mode doubles vertical resolution using block characters.
	content, err := gopixels.FromImageStream(img, widthCells, 0, "halfcell", true)
	if err != nil || maxHeightCells <= 0 {
		return content, err
	}
	lines := strings.Split(content, "\n")
	if len(lines) > maxHeightCells {
		content = strings.Join(lines[:maxHeightCells], "\n")
	}
	return content, nil
}

// renderKitty decodes and resizes the image for kitty transmission and
// returns the PNG bytes plus the display dimensions in cells.
func renderKitty(data []byte, itemID string, requestID uint64, widthCells, maxHeightCells int) (pngBytes []byte, imageID uint32, wCells, hCells int, err error) {
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
	if maxHeightCells > 0 && dispH > maxHeightCells*kittyCellHpx {
		maxWidth := (maxHeightCells * kittyCellHpx * b.Dx()) / b.Dy()
		widthCells = maxWidth / kittyCellWpx
		if widthCells < 1 {
			widthCells = 1
		}
		dispW = widthCells * kittyCellWpx
		dispH = (dispW * b.Dy()) / b.Dx()
	}
	if dispH < 1 {
		dispH = 1
	}
	resized := resizeNearest(img, dispW, dispH)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return nil, 0, 0, 0, err
	}

	heightCells := (dispH + kittyCellHpx - 1) / kittyCellHpx
	if heightCells < 1 {
		heightCells = 1
	}
	if maxHeightCells > 0 && heightCells > maxHeightCells {
		heightCells = maxHeightCells
	}
	return buf.Bytes(), kittyImageID(itemID, requestID), widthCells, heightCells, nil
}

// kittyImageID returns a request-specific ID for fetched posters. Using a new
// ID for every request prevents the terminal from reusing stale image data.
// requestID is zero for the standalone RenderPoster helper, where an item
// stable ID is still useful.
func kittyImageID(itemID string, requestID uint64) uint32 {
	if requestID != 0 {
		id := uint32(requestID)
		if id != 0 {
			return id
		}
	}
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
			fmt.Fprintf(&sb, "a=t,f=100,i=%d,q=2", imageID)
			first = false
		} else {
			sb.WriteString("q=2")
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

// kittyPlacement returns a virtual kitty placement. The image is displayed
// when the host emits matching placeholders for its cells.
func kittyPlacement(imageID uint32, widthCells, heightCells int) string {
	var sb strings.Builder
	sb.WriteString("\x1b_G")
	fmt.Fprintf(&sb, "a=p,U=1,i=%d,c=%d,r=%d,q=2", imageID, widthCells, heightCells)
	sb.WriteString("\x1b\\")
	return sb.String()
}

// kittyPlaceholders identifies each cell of a virtual kitty placement. Kitty
// uses the foreground color for the low 24 image-ID bits and combining
// diacritics for the row, column, and optional high ID byte.
func kittyPlaceholders(imageID uint32, widthCells, heightCells int) string {
	var sb strings.Builder
	red := byte(imageID >> 16)
	green := byte(imageID >> 8)
	blue := byte(imageID)
	highByte := int(imageID >> 24)

	fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm", red, green, blue)
	for row := 0; row < heightCells; row++ {
		for col := 0; col < widthCells; col++ {
			sb.WriteRune(kitty.Placeholder)
			sb.WriteRune(kitty.Diacritic(row))
			sb.WriteRune(kitty.Diacritic(col))
			if highByte != 0 {
				sb.WriteRune(kitty.Diacritic(highByte))
			}
		}
		if row < heightCells-1 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\x1b[39m")
	return sb.String()
}

// deleteKittyImage removes a previously uploaded image from the terminal.
// This prevents an old virtual placement from remaining visible when a new
// selection has no poster or while its replacement is loading.
func deleteKittyImage(output io.Writer, imageID uint32) {
	if imageID == 0 {
		return
	}
	if output == nil {
		output = os.Stdout
	}
	_, _ = io.WriteString(output, fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", imageID))
}

// FetchPosterCmd returns a command that downloads and renders the poster for
// the given item, emitting a PosterLoadedMsg on success.
func FetchPosterCmd(client mediaserver.MediaSource, output io.Writer, requestID uint64, itemID, url string, widthCells, maxHeightCells int) tea.Cmd {
	if client == nil || (url == "" && itemID == "") {
		return nil
	}
	return func() tea.Msg {
		data, err := fetchPosterData(client, itemID, url)
		if err != nil {
			slog.Debug("poster fetch failed", "itemID", itemID, "url", url, "error", err)
			return nil
		}

		if SupportsKittyImage() {
			pngBytes, imageID, w, h, err := renderKitty(data, itemID, requestID, widthCells, maxHeightCells)
			if err == nil {
				// Transmit the pixels once. The placement + placeholders are
				// rendered in the View; on subsequent frames only placeholders
				// are re-emitted to keep the image visible without re-upload.
				if output == nil {
					output = os.Stdout
				}
				if _, err := io.WriteString(output, kittyTransmit(pngBytes, imageID)); err != nil {
					slog.Debug("kitty transmit failed", "itemID", itemID, "error", err)
					return nil
				}
				return PosterLoadedMsg{
					RequestID: requestID,
					ItemID:    itemID,
					Content:   kittyPlaceholders(imageID, w, h),
					Placement: kittyPlacement(imageID, w, h),
					ImageID:   imageID,
				}
			}
			slog.Debug("kitty render failed, falling back to ASCII", "itemID", itemID, "error", err)
		}

		content, err := renderASCII(data, widthCells, maxHeightCells)
		if err != nil || content == "" {
			slog.Debug("ASCII render failed", "itemID", itemID, "error", err, "empty", content == "")
			return nil
		}
		return PosterLoadedMsg{RequestID: requestID, ItemID: itemID, Content: content}
	}
}

// fetchPosterData retries short-lived network failures and bounds each HTTP
// request so a stalled image server cannot leave the poster permanently stuck.
func fetchPosterData(client mediaserver.MediaSource, itemID, url string) ([]byte, error) {
	requestURL := url
	if requestURL == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		item, err := client.GetMediaItem(ctx, itemID)
		cancel()
		if err != nil {
			return nil, err
		}
		requestURL = PosterURL(item)
		if requestURL == "" {
			return nil, fmt.Errorf("no poster URL for item %s", itemID)
		}
	}

	var data []byte
	var err error
	for attempt := 0; attempt < posterFetchAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		data, err = client.GetImage(ctx, requestURL)
		cancel()
		if err == nil {
			return data, nil
		}
		slog.Debug("fetchPosterData download attempt failed", "itemID", itemID, "attempt", attempt+1, "error", err)
		if attempt+1 < posterFetchAttempts {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil, err
}
