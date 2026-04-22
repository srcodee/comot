package progress

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const barWidth = 24

type Tracker struct {
	out       io.Writer
	started   time.Time
	total     int
	completed int
	current   string
	lastWidth int
	enabled   bool
	active    bool
	mu        sync.Mutex
}

func New(out io.Writer, enabled bool) *Tracker {
	return &Tracker{
		out:     out,
		started: time.Now(),
		total:   1,
		enabled: enabled,
	}
}

func (t *Tracker) AddTotal(delta int) {
	if !t.enabled || delta <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.total += delta
	t.renderLocked()
}

func (t *Tracker) Start(label string) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.current = label
	t.active = true
	t.renderLocked()
}

func (t *Tracker) Advance() {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.completed++
	if t.completed > t.total {
		t.total = t.completed
	}
	t.active = true
	t.renderLocked()
}

func (t *Tracker) Finish() {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.completed = t.total
	t.current = "done"
	t.renderLocked()
	fmt.Fprint(t.out, "\n")
	t.active = false
	t.lastWidth = 0
}

func (t *Tracker) BeforeOutput() {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.active {
		return
	}
	fmt.Fprint(t.out, "\r\033[2K")
	t.lastWidth = 0
	t.active = false
}

func (t *Tracker) renderLocked() {
	total := t.total
	if total <= 0 {
		total = 1
	}
	completed := t.completed
	if completed > total {
		completed = total
	}

	filled := completed * barWidth / total
	bar := strings.Repeat("=", filled)
	if filled < barWidth {
		bar += ">"
		bar += strings.Repeat(".", barWidth-filled-1)
	}

	if filled >= barWidth {
		bar = strings.Repeat("=", barWidth)
	}

	line := fmt.Sprintf(
		"\r\033[2K[%s] [%s] %d/%d elapsed %s current %s",
		bar,
		time.Now().Format("15:04:05"),
		completed,
		total,
		time.Since(t.started).Round(100*time.Millisecond),
		trim(t.current, 56),
	)

	if pad := t.lastWidth - len(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	t.lastWidth = len(line)
	fmt.Fprint(t.out, line)
}

func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
