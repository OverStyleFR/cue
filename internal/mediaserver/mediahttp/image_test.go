package mediahttp

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadImageRejectsOversizedResponses(t *testing.T) {
	for _, contentLength := range []int64{maxImageBytes + 1, -1} {
		resp := &http.Response{
			ContentLength: contentLength,
			Body:          io.NopCloser(strings.NewReader(strings.Repeat("x", int(maxImageBytes+1)))),
		}
		if _, err := ReadImage(resp); err == nil {
			t.Fatalf("Content-Length %d: expected oversized response error", contentLength)
		}
	}
}
