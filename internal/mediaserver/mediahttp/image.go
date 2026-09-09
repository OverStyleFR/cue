package mediahttp

import (
	"fmt"
	"io"
	"net/http"
)

const maxImageBytes int64 = 20 << 20

// ReadImage rejects oversized image responses without trusting Content-Length.
func ReadImage(resp *http.Response) ([]byte, error) {
	if resp.ContentLength > maxImageBytes {
		return nil, fmt.Errorf("image response exceeds %d bytes", maxImageBytes)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxImageBytes {
		return nil, fmt.Errorf("image response exceeds %d bytes", maxImageBytes)
	}
	return data, nil
}
