package ux

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// plainProgressInterval is the minimum time between percentage lines so
// piped output is not spammed. The first update and the 100% line always
// emit, regardless of this interval.
const plainProgressInterval = 200 * time.Millisecond

// plain is the ASCII reporter used when color is resolved off. It never
// writes ANSI escape sequences.
type plain struct {
	// w receives unstyled lines.
	w io.Writer
	// progress is true when percentage lines should be written.
	progress bool
	// mu guards rate-limit state and writes.
	mu sync.Mutex
	// last is the time of the last emitted percentage line.
	last time.Time
	// lastPct is the last emitted percentage, or -1 before the first line.
	lastPct int
	// emitted is true after at least one percentage line in this step.
	emitted bool
}

// newPlain constructs an unstyled reporter writing to w.
func newPlain(w io.Writer, progress bool) *plain {
	return &plain{w: w, progress: progress, lastPct: -1}
}

// Step prints an ASCII step header.
func (p *plain) Step(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resetProgressLocked()
	writeString(p.w, "==> "+name+"\n")
}

// Progress writes a rate-limited percentage line when progress is enabled.
// Calls are ignored when progress is resolved off, so piped builds stay quiet.
func (p *plain) Progress(done, total int64) {
	if !p.progress {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	pct, _ := progressParts(done, total)
	complete := total > 0 && done >= total
	if !p.shouldEmitLocked(pct, complete) {
		return
	}
	p.last = time.Now()
	p.lastPct = pct
	p.emitted = true
	writeString(p.w, fmt.Sprintf("progress %d%% (%d/%d)\n", pct, done, total))
}

// Done prints an ASCII completion line.
func (p *plain) Done(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resetProgressLocked()
	writeString(p.w, "done "+name+"\n")
}

// shouldEmitLocked reports whether this progress tick should become a line.
// Caller holds p.mu.
func (p *plain) shouldEmitLocked(pct int, complete bool) bool {
	if complete {
		return !p.emitted || p.lastPct != percentScale
	}
	if !p.emitted {
		return true
	}
	if pct == p.lastPct {
		return false
	}
	return time.Since(p.last) >= plainProgressInterval
}

// resetProgressLocked clears rate-limit state at step boundaries.
func (p *plain) resetProgressLocked() {
	p.last = time.Time{}
	p.lastPct = -1
	p.emitted = false
}
