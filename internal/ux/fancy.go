package ux

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

const (
	// fancyCheck is the completed-step mark.
	fancyCheck = "✓"
	// fancyStepMark prefixes a live step header.
	fancyStepMark = "▸"
	// progressBarSize is the number of cells in the fancy progress bar.
	progressBarSize = 20
	// percentScale is 100%, used for both renderers.
	percentScale = 100
)

// SummaryRow is one label/value pair in a [Summary] block.
type SummaryRow struct {
	// Label is the left-hand field name.
	Label string
	// Value is the right-hand field value.
	Value string
}

// VersionRow is one release row for [VersionsTable].
type VersionRow struct {
	// Version is the update version name.
	Version string
	// Channel is the release channel.
	Channel string
	// Architecture is the CPU architecture.
	Architecture string
	// Type is the image type (iso or raw).
	Type string
}

// fancy is the Lip Gloss reporter used when color is resolved on.
type fancy struct {
	// w receives styled output. When constructed via [New] with
	// ColorModeAlways this is a colorprofile.Writer pinned to TrueColor.
	w io.Writer
	// progress is true when progress lines should be redrawn.
	progress bool
	// pal is the shared Lip Gloss palette.
	pal Palette
	// mu guards step/open/lastLen and writes.
	mu sync.Mutex
	// step is the name of the in-flight step, used on the progress line.
	step string
	// open is true while a carriage-return progress line is on screen.
	open bool
	// lastLen is the visible width of the last progress line, for padding.
	lastLen int
}

// newFancy constructs a styled reporter writing to w.
func newFancy(w io.Writer, progress bool, pal Palette) *fancy {
	return &fancy{w: w, progress: progress, pal: pal}
}

// Step prints a styled step header, clearing any in-place progress line.
func (f *fancy) Step(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearProgressLocked()
	f.step = name
	line := f.pal.Step.Render(fancyStepMark + " " + name)
	writeString(f.w, line+"\n")
}

// Progress redraws a single carriage-return progress line when progress is
// enabled. No output is written when progress is resolved off.
func (f *fancy) Progress(done, total int64) {
	if !f.progress {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	line := f.pal.Progress.Render(formatProgressBar(f.step, done, total))
	f.redrawLocked(line)
}

// Done prints a styled checkmark line, clearing any in-place progress line.
func (f *fancy) Done(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearProgressLocked()
	line := f.pal.Done.Render(fancyCheck + " " + name)
	writeString(f.w, line+"\n")
}

// redrawLocked writes line prefixed with CR, padding to erase a longer
// previous line. Caller holds f.mu.
func (f *fancy) redrawLocked(line string) {
	visible := lipgloss.Width(line)
	pad := 0
	if f.lastLen > visible {
		pad = f.lastLen - visible
	}
	writeString(f.w, "\r"+line+strings.Repeat(" ", pad))
	f.lastLen = visible
	f.open = true
}

// clearProgressLocked erases an open progress line. Caller holds f.mu.
func (f *fancy) clearProgressLocked() {
	if !f.open {
		return
	}
	writeString(f.w, "\r"+strings.Repeat(" ", f.lastLen)+"\r")
	f.open = false
	f.lastLen = 0
}

// formatProgressBar returns the unstyled progress text for a CR redraw.
func formatProgressBar(step string, done, total int64) string {
	pct, filled := progressParts(done, total)
	bar := strings.Repeat("=", filled) + strings.Repeat("-", progressBarSize-filled)
	name := step
	if name == "" {
		name = "progress"
	}
	return fmt.Sprintf("%s [%s] %3d%%", name, bar, pct)
}

// progressParts returns a 0–100 percentage and filled bar cells.
func progressParts(done, total int64) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	pct := int(done * int64(percentScale) / total)
	filled := int(done * int64(progressBarSize) / total)
	if done == total {
		pct = percentScale
		filled = progressBarSize
	}
	return pct, filled
}

// Summary writes a bordered label/value block to w. Lip Gloss v2 downsamples
// colors from the writer and environment via [lipgloss.Fprintln].
func Summary(w io.Writer, rows []SummaryRow) {
	if w == nil {
		return
	}
	pal := DefaultPalette()
	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, pal.SummaryTitle.Render("summary"))
	for _, row := range rows {
		key := pal.SummaryKey.Render(row.Label)
		val := pal.SummaryValue.Render(row.Value)
		lines = append(lines, key+"  "+val)
	}
	block := pal.SummaryBox.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	_, err := lipgloss.Fprintln(w, block)
	if err != nil {
		return
	}
}

// VersionsTable renders rows as a Lip Gloss table for the Phase 4 versions
// command. The returned string is full-fidelity ANSI; callers print it with
// [lipgloss.Fprint] so the writer-based color profile can downsample.
func VersionsTable(rows []VersionRow) string {
	pal := DefaultPalette()
	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(pal.TableBorder).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return pal.TableHeader
			}
			return pal.TableCell
		}).
		Headers("Version", "Channel", "Architecture", "Type")
	for _, row := range rows {
		tbl.Row(row.Version, row.Channel, row.Architecture, row.Type)
	}
	return tbl.String()
}
