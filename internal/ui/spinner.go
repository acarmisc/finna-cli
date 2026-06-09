package ui

import (
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/term"
)

// Spinner provides a minimal progress indicator. When stdout is not a TTY
// (e.g. CI, pipe) it writes a plain "label... " line and nothing more.
type Spinner struct {
	label  string
	w      io.Writer
	tty    bool
	done   chan struct{}
	frames []string
}

var spinFrames = []string{"⠋", "⠙", "⠸", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Start creates and starts a Spinner. Call Stop/StopWithSuccess/StopWithError
// when the operation completes.
func Start(label string) *Spinner {
	s := &Spinner{
		label:  label,
		w:      os.Stderr,
		tty:    term.IsTerminal(int(os.Stderr.Fd())),
		done:   make(chan struct{}),
		frames: spinFrames,
	}
	if s.tty {
		go s.animate()
	} else {
		fmt.Fprintf(s.w, "%s... ", label)
	}
	return s
}

func (s *Spinner) animate() {
	i := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			fmt.Fprintf(s.w, "\r%s %s", s.frames[i%len(s.frames)], s.label)
			i++
		}
	}
}

func (s *Spinner) clear() {
	if s.tty {
		// Erase current line.
		fmt.Fprintf(s.w, "\r\033[K")
	}
}

// Stop halts the spinner without printing a result message.
func (s *Spinner) Stop() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	s.clear()
}

// StopWithSuccess halts and prints a success message.
func (s *Spinner) StopWithSuccess(msg string) {
	s.Stop()
	if s.tty {
		fmt.Fprintf(s.w, "✓ %s\n", msg)
	} else {
		fmt.Fprintf(s.w, "ok — %s\n", msg)
	}
}

// StopWithError halts and prints an error message.
func (s *Spinner) StopWithError(msg string) {
	s.Stop()
	if s.tty {
		fmt.Fprintf(s.w, "✗ %s\n", msg)
	} else {
		fmt.Fprintf(s.w, "error — %s\n", msg)
	}
}
