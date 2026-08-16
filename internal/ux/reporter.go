package ux

import (
	"io"
	"os"

	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"

	"github.com/componere/incusos-builder/internal/build"
)

// ColorMode selects whether the reporter emits ANSI styles.
type ColorMode string

const (
	// ColorModeAuto styles only when w is a TTY and NO_COLOR/TERM=dumb are unset.
	ColorModeAuto ColorMode = "auto"
	// ColorModeAlways forces ANSI even when w is a capture buffer.
	ColorModeAlways ColorMode = "always"
	// ColorModeNever strips all styling.
	ColorModeNever ColorMode = "never"
)

// ProgressMode selects whether the reporter emits progress updates.
type ProgressMode string

const (
	// ProgressModeAuto enables progress when w is a TTY.
	ProgressModeAuto ProgressMode = "auto"
	// ProgressModeAlways emits progress even when w is not a TTY.
	ProgressModeAlways ProgressMode = "always"
	// ProgressModeNever suppresses progress (and therefore any progress ANSI).
	ProgressModeNever ProgressMode = "never"
)

// New returns a [build.Reporter] writing to w. Nil w is treated as
// [io.Discard]. Color and progress modes are resolved as documented on
// the package.
func New(color ColorMode, progress ProgressMode, w io.Writer) build.Reporter {
	if w == nil {
		w = io.Discard
	}
	enableProgress := resolveProgress(progress, w)
	if resolveColor(color, w) {
		return newFancy(colorWriter(color, w), enableProgress, DefaultPalette())
	}
	return newPlain(w, enableProgress)
}

// NewLogger returns a charm log/v2 logger writing to w, styled from
// [DefaultPalette]. It is the only logger the CLI should use. Color
// resolution matches [New]: always forces a TrueColor profile even to a
// buffer; never/auto-off pin the profile to NoTTY so styling is stripped.
func NewLogger(color ColorMode, w io.Writer) *log.Logger {
	if w == nil {
		w = io.Discard
	}
	logger := log.New(w)
	logger.SetStyles(logStyles(DefaultPalette()))
	if resolveColor(color, w) {
		if color == ColorModeAlways {
			// Lip Gloss / Log v2 color is writer-based; Detect would see a
			// buffer as NoTTY. Always overrides that for tests and --color=always.
			logger.SetColorProfile(colorprofile.TrueColor)
		}
		return logger
	}
	logger.SetColorProfile(colorprofile.NoTTY)
	return logger
}

// colorWriter wraps w so Lip Gloss v2 downsampling happens at the output
// layer. ColorModeAlways pins TrueColor even when w is not a TTY.
func colorWriter(color ColorMode, w io.Writer) io.Writer {
	out := colorprofile.NewWriter(w, os.Environ())
	if color == ColorModeAlways {
		out.Profile = colorprofile.TrueColor
	}
	return out
}

// resolveColor reports whether ANSI styling is enabled for color on w.
func resolveColor(color ColorMode, w io.Writer) bool {
	switch color {
	case ColorModeAlways:
		return true
	case ColorModeNever:
		return false
	case ColorModeAuto:
		return colorAutoEnabled(w)
	default:
		return colorAutoEnabled(w)
	}
}

// resolveProgress reports whether progress events should be rendered.
func resolveProgress(progress ProgressMode, w io.Writer) bool {
	switch progress {
	case ProgressModeAlways:
		return true
	case ProgressModeNever:
		return false
	case ProgressModeAuto:
		return isTTY(w)
	default:
		return isTTY(w)
	}
}

// colorAutoEnabled implements ColorModeAuto precedence after explicit modes:
// NO_COLOR, then TERM=dumb, then TTY detection on w.
func colorAutoEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTTY(w)
}

// isTTY reports whether w is a terminal. Capture buffers and wrapped
// writers without Fd are not TTYs.
func isTTY(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(file.Fd())
}

// writeString writes s to w, ignoring errors because [build.Reporter]
// has no error return.
func writeString(w io.Writer, s string) {
	_, err := io.WriteString(w, s)
	if err != nil {
		return
	}
}
