package tui

import (
	"io"
	"sync"
)

// OutputWriter serializes terminal writes from Bubble Tea and asynchronous
// poster fetch commands. Kitty image chunks must reach the terminal contiguously.
type OutputWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewOutputWriter wraps a terminal writer for synchronized access.
func NewOutputWriter(w io.Writer) *OutputWriter {
	return &OutputWriter{w: w}
}

// Write serializes writes to the wrapped terminal.
func (w *OutputWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

// Fd preserves Bubble Tea's terminal detection when the wrapped writer is a
// terminal file.
func (w *OutputWriter) Fd() uintptr {
	if f, ok := w.w.(interface{ Fd() uintptr }); ok {
		return f.Fd()
	}
	return 0
}
