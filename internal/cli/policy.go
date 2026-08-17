package cli

import (
	"io"
	"os"

	charmlog "charm.land/log/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/componere/incusos-builder/internal/ux"
)

const (
	// envCI is the conventional CI indicator that auto-enables --no-input.
	envCI = "CI"
	// modeChoices is the accepted set for --color and --progress.
	modeChoices = "auto, always, or never"
)

// policy is the resolved interaction and tool-config policy for one invocation.
type policy struct {
	// NoInput disables every prompt, including auto-on for non-TTY/CI.
	NoInput bool
	// Color is passed through to ux; AUTO is not pre-resolved here.
	Color ux.ColorMode
	// Progress is passed to ux; AUTO that fails the both-streams rule is Never.
	Progress ux.ProgressMode
	// Verbose selects DebugLevel on Log.
	Verbose bool
	// Quiet suppresses non-error output.
	Quiet bool
	// JSON selects the single-envelope stdout contract.
	JSON bool
	// Server is the update server URL or local mirror directory.
	Server string
	// CacheDir is the content-addressed download cache directory.
	CacheDir string
	// Log writes diagnostics to stderr. Debug when Verbose, error-only
	// when Quiet, warn-and-above otherwise.
	Log *charmlog.Logger
}

// resolvePolicy reads Viper-backed settings plus verbose/quiet flags and
// applies TTY/CI auto-on rules.
func resolvePolicy(cmd *cobra.Command, opts Options, vp *viper.Viper) (policy, error) {
	color, err := parseColor(vp.GetString(flagColor))
	if err != nil {
		return policy{}, err
	}
	progress, err := parseProgress(vp.GetString(flagProgress), stdoutIsTTY(opts), stderrIsTTY(opts))
	if err != nil {
		return policy{}, err
	}
	verbose, err := cmd.Flags().GetBool(flagVerbose)
	if err != nil {
		return policy{}, err
	}
	quiet, err := cmd.Flags().GetBool(flagQuiet)
	if err != nil {
		return policy{}, err
	}
	if verbose && quiet {
		return policy{}, usagef("--verbose and -q are mutually exclusive")
	}
	return policy{
		NoInput:  vp.GetBool(flagNoInput) || autoNoInput(opts),
		Color:    color,
		Progress: progress,
		Verbose:  verbose,
		Quiet:    quiet,
		JSON:     vp.GetBool(flagJSON),
		Server:   vp.GetString(flagServer),
		CacheDir: vp.GetString(flagCacheDir),
		Log:      newPolicyLogger(color, verbose, quiet, opts.Err),
	}, nil
}

// newPolicyLogger constructs the stderr logger for one invocation.
func newPolicyLogger(color ux.ColorMode, verbose, quiet bool, errW io.Writer) *charmlog.Logger {
	logger := ux.NewLogger(color, errW)
	switch {
	case verbose:
		logger.SetLevel(charmlog.DebugLevel)
	case quiet:
		logger.SetLevel(charmlog.ErrorLevel)
	default:
		logger.SetLevel(charmlog.WarnLevel)
	}
	return logger
}

// parseColor validates --color.
func parseColor(raw string) (ux.ColorMode, error) {
	mode := ux.ColorMode(raw)
	switch mode {
	case ux.ColorModeAuto, ux.ColorModeAlways, ux.ColorModeNever:
		return mode, nil
	default:
		return "", usagef("invalid --color %q (want %s)", raw, modeChoices)
	}
}

// parseProgress validates --progress and pre-resolves AUTO against both streams.
func parseProgress(raw string, stdoutTTY, stderrTTY bool) (ux.ProgressMode, error) {
	mode := ux.ProgressMode(raw)
	switch mode {
	case ux.ProgressModeAlways, ux.ProgressModeNever:
		return mode, nil
	case ux.ProgressModeAuto:
		if stdoutTTY && stderrTTY {
			return ux.ProgressModeAuto, nil
		}
		return ux.ProgressModeNever, nil
	default:
		return "", usagef("invalid --progress %q (want %s)", raw, modeChoices)
	}
}

// autoNoInput is true when stdin or stdout is not a TTY, or CI is set.
func autoNoInput(opts Options) bool {
	if os.Getenv(envCI) != "" {
		return true
	}
	return !stdinIsTTY(opts) || !stdoutIsTTY(opts)
}

// stdinIsTTY reports whether stdin is a terminal.
func stdinIsTTY(opts Options) bool {
	if opts.StdinTTY != nil {
		return opts.StdinTTY()
	}
	return isReaderTTY(opts.In)
}

// stdoutIsTTY reports whether stdout is a terminal.
func stdoutIsTTY(opts Options) bool {
	if opts.StdoutTTY != nil {
		return opts.StdoutTTY()
	}
	return isWriterTTY(opts.Out)
}

// stderrIsTTY reports whether stderr is a terminal.
func stderrIsTTY(opts Options) bool {
	if opts.StderrTTY != nil {
		return opts.StderrTTY()
	}
	return isWriterTTY(opts.Err)
}

// isReaderTTY reports whether r is an [os.File] terminal.
func isReaderTTY(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(file.Fd())
}

// isWriterTTY reports whether w is an [os.File] terminal.
func isWriterTTY(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(file.Fd())
}
