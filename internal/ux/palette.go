package ux

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
)

const (
	// colorAccent is the primary brand hex used for steps and info.
	colorAccent = "#874BFD"
	// colorSuccess is the completed-step hex.
	colorSuccess = "#02BA84"
	// colorWarn is the warning hex.
	colorWarn = "#D4A017"
	// colorError is the error hex.
	colorError = "#FF5F87"
	// colorFatal is the fatal-level hex.
	colorFatal = "#C084FC"
	// colorDebug is the debug-level hex.
	colorDebug = "#5B8DEF"
	// colorSubtle is the muted/secondary hex.
	colorSubtle = "#6C7086"
)

// Palette is the shared Lip Gloss color and style set.
type Palette struct {
	// Accent is the primary brand color (steps, info).
	Accent color.Color
	// Success is the completed-step / info-ok color.
	Success color.Color
	// Warn is the warning color.
	Warn color.Color
	// Error is the error color.
	Error color.Color
	// Fatal is the fatal-level color.
	Fatal color.Color
	// Debug is the debug-level color.
	Debug color.Color
	// Subtle is the secondary/muted color (keys, borders).
	Subtle color.Color
	// Step styles a step header.
	Step lipgloss.Style
	// Done styles a completed-step line.
	Done lipgloss.Style
	// Progress styles the in-place progress line.
	Progress lipgloss.Style
	// SummaryTitle styles the summary card title.
	SummaryTitle lipgloss.Style
	// SummaryKey styles a summary label.
	SummaryKey lipgloss.Style
	// SummaryValue styles a summary value.
	SummaryValue lipgloss.Style
	// SummaryBox styles the summary card frame.
	SummaryBox lipgloss.Style
	// TableHeader styles versions-table header cells.
	TableHeader lipgloss.Style
	// TableCell styles versions-table data cells.
	TableCell lipgloss.Style
	// TableBorder styles versions-table borders.
	TableBorder lipgloss.Style
}

// DefaultPalette returns the shared palette. Styles are values; callers may
// copy and tweak them without affecting other users.
func DefaultPalette() Palette {
	accent := lipgloss.Color(colorAccent)
	success := lipgloss.Color(colorSuccess)
	warn := lipgloss.Color(colorWarn)
	errColor := lipgloss.Color(colorError)
	fatal := lipgloss.Color(colorFatal)
	debug := lipgloss.Color(colorDebug)
	subtle := lipgloss.Color(colorSubtle)

	return Palette{
		Accent:  accent,
		Success: success,
		Warn:    warn,
		Error:   errColor,
		Fatal:   fatal,
		Debug:   debug,
		Subtle:  subtle,
		Step: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),
		Done: lipgloss.NewStyle().
			Foreground(success).
			Bold(true),
		Progress: lipgloss.NewStyle().
			Foreground(accent),
		SummaryTitle: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),
		SummaryKey: lipgloss.NewStyle().
			Foreground(subtle),
		SummaryValue: lipgloss.NewStyle().
			Foreground(success),
		SummaryBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1),
		TableHeader: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true).
			Align(lipgloss.Center).
			Padding(0, 1),
		TableCell: lipgloss.NewStyle().
			Padding(0, 1),
		TableBorder: lipgloss.NewStyle().
			Foreground(subtle),
	}
}

// logStyles returns charm log TextFormatter styles painted from p.
func logStyles(p Palette) *log.Styles {
	styles := log.DefaultStyles()
	styles.Levels[log.DebugLevel] = lipgloss.NewStyle().
		SetString("DEBU").
		Bold(true).
		Foreground(p.Debug)
	styles.Levels[log.InfoLevel] = lipgloss.NewStyle().
		SetString("INFO").
		Bold(true).
		Foreground(p.Accent)
	styles.Levels[log.WarnLevel] = lipgloss.NewStyle().
		SetString("WARN").
		Bold(true).
		Foreground(p.Warn)
	styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().
		SetString("ERRO").
		Bold(true).
		Foreground(p.Error)
	styles.Levels[log.FatalLevel] = lipgloss.NewStyle().
		SetString("FATA").
		Bold(true).
		Foreground(p.Fatal)
	styles.Key = lipgloss.NewStyle().Foreground(p.Subtle)
	return styles
}
