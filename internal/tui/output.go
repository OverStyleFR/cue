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

var _ interface {
	io.ReadWriteCloser
	Fd() uintptr
} = (*OutputWriter)(nil)

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

// Read satisfies Bubble Tea's terminal-file interface. Output-only writers do
// not provide input, so reads return EOF.
func (w *OutputWriter) Read(p []byte) (int, error) {
	if r, ok := w.w.(io.Reader); ok {
		return r.Read(p)
	}
	return 0, io.EOF
}

// Close keeps ownership of the wrapped output with the caller.
func (w *OutputWriter) Close() error {
	return nil
}

// Fd preserves Bubble Tea's terminal detection when the wrapped writer is a
// terminal file.
func (w *OutputWriter) Fd() uintptr {
	if f, ok := w.w.(interface{ Fd() uintptr }); ok {
		return f.Fd()
	}
	return 0
}
