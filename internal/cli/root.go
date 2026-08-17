package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/componere/incusos-builder/internal/ux"
)

const (
	// appName is the binary / cobra Use string.
	appName = "incusos-builder"
	// envPrefix is the Viper environment prefix (INCUSOS_BUILDER_*).
	envPrefix = "INCUSOS_BUILDER"
	// defaultServer is the default --server URL.
	defaultServer = "https://images.linuxcontainers.org/os"

	flagColor    = "color"
	flagProgress = "progress"
	flagNoInput  = "no-input"
	flagVerbose  = "verbose"
	flagQuiet    = "quiet"
	flagServer   = "server"
	flagCacheDir = "cache-dir"
	flagJSON     = "json"
)

// BuildInfo describes linker-injected build metadata printed by --version.
type BuildInfo struct {
	// Version is the first --version token.
	Version string
	// Commit is the source commit used to build the binary.
	Commit string
	// Date is the build timestamp.
	Date string
}

// Options customizes root command construction.
type Options struct {
	// In receives interactive command input.
	In io.Reader
	// Out receives machine-readable command output.
	Out io.Writer
	// Err receives diagnostics and human-readable status.
	Err io.Writer
	// Build controls the first --version line.
	Build BuildInfo
	// IncusOSPin is printed as-is on the second --version line
	// (`incus-os API: …`).
	IncusOSPin string
	// Viper is the configuration instance used by the command tree.
	Viper *viper.Viper
	// StdinTTY reports whether stdin is a terminal. Nil uses Fd detection on In.
	StdinTTY func() bool
	// StdoutTTY reports whether stdout is a terminal. Nil uses Fd detection on Out.
	StdoutTTY func() bool
	// StderrTTY reports whether stderr is a terminal. Nil uses Fd detection on Err.
	StderrTTY func() bool
}

// NewRootCommand creates the incusos-builder Cobra command tree.
func NewRootCommand(options Options) *cobra.Command {
	options = options.withDefaults()

	root := &cobra.Command{
		Use:           appName,
		Short:         "Build seeded IncusOS installation media from a YAML config",
		Version:       options.Build.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          noArgs(options),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := initializeConfig(cmd, options.Viper); err != nil {
				return reportBuildError(options, jsonRequested(cmd, options), err)
			}
			if _, err := resolvePolicy(cmd, options, options.Viper); err != nil {
				return reportBuildError(options, jsonRequested(cmd, options), err)
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	root.SetVersionTemplate(
		fmt.Sprintf(
			"%s %s (%s) built %s\nincus-os API: %s\n",
			appName,
			options.Build.Version,
			options.Build.Commit,
			options.Build.Date,
			options.IncusOSPin,
		),
	)
	root.SetIn(options.In)
	root.SetOut(options.Out)
	root.SetErr(options.Err)
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return reportBuildError(options, jsonRequested(cmd, options), flagUsageError(cmd, err))
	})
	registerPersistentFlags(root)
	root.AddCommand(
		newBuildCommand(options),
		newValidateCommand(options),
		newVersionsCommand(options),
		newInitCommand(options),
	)
	return root
}

// withDefaults fills nil IO/Viper fields and placeholder build metadata.
func (o Options) withDefaults() Options {
	if o.In == nil {
		o.In = strings.NewReader("")
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	if o.Err == nil {
		o.Err = io.Discard
	}
	if o.Viper == nil {
		o.Viper = viper.New()
	}
	o.Build = o.Build.withDefaults()
	return o
}

// withDefaults fills empty build metadata fields with placeholder values.
func (b BuildInfo) withDefaults() BuildInfo {
	if strings.TrimSpace(b.Version) == "" {
		b.Version = "dev"
	}
	if strings.TrimSpace(b.Commit) == "" {
		b.Commit = "none"
	}
	if strings.TrimSpace(b.Date) == "" {
		b.Date = "unknown"
	}
	return b
}

// registerPersistentFlags installs the persistent --color, --progress,
// --no-input, --verbose, -q/--quiet, --server, --cache-dir, and --json flags.
func registerPersistentFlags(root *cobra.Command) {
	pf := root.PersistentFlags()
	pf.String(flagColor, string(ux.ColorModeAuto), "color output: auto, always, or never")
	pf.String(flagProgress, string(ux.ProgressModeAuto), "progress line: auto, always, or never")
	pf.Bool(flagNoInput, false, "disable all prompts")
	pf.Bool(flagVerbose, false, "enable verbose logging")
	pf.BoolP(flagQuiet, "q", false, "suppress non-error output")
	pf.String(flagServer, defaultServer, "update server URL or local mirror directory")
	pf.String(flagCacheDir, defaultCacheDir(), "content-addressed download cache directory")
	pf.Bool(flagJSON, false, "write a single JSON envelope to stdout")
}

// initializeConfig wires Viper after flags are parsed (parsed-flag pattern).
//
// Only flags the user actually passed are bound, so cobra defaults cannot
// mask INCUSOS_BUILDER_* environment variables. Read resolved values through
// Viper, not the original flag variables.
func initializeConfig(cmd *cobra.Command, vp *viper.Viper) error {
	vp.SetEnvPrefix(envPrefix)
	vp.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	// Empty INCUSOS_BUILDER_* values are equivalent to passing an empty flag.
	// Viper otherwise drops them and falls through to defaults.
	vp.AllowEmptyEnv(true)
	vp.AutomaticEnv()
	applyViperDefaults(vp)
	return bindParsedFlags(cmd, vp)
}

// applyViperDefaults registers the six Viper-backed keys so env lookup and
// Get* have somewhere to fall through when the matching flag was not parsed.
func applyViperDefaults(vp *viper.Viper) {
	vp.SetDefault(flagColor, string(ux.ColorModeAuto))
	vp.SetDefault(flagProgress, string(ux.ProgressModeAuto))
	vp.SetDefault(flagNoInput, false)
	vp.SetDefault(flagServer, defaultServer)
	vp.SetDefault(flagCacheDir, defaultCacheDir())
	vp.SetDefault(flagJSON, false)
}

// bindParsedFlags binds Viper-backed flags that were set on the command line.
func bindParsedFlags(cmd *cobra.Command, vp *viper.Viper) error {
	names := []string{flagServer, flagCacheDir, flagJSON, flagColor, flagProgress, flagNoInput}
	flags := cmd.Flags()
	for _, name := range names {
		f := flags.Lookup(name)
		if f == nil || !f.Changed {
			continue
		}
		if err := vp.BindPFlag(name, f); err != nil {
			return fmt.Errorf("bind flag %s: %w", name, err)
		}
	}
	return nil
}

// defaultCacheDir returns $XDG_CACHE_HOME/incusos-builder, or empty if the
// user cache directory cannot be determined.
func defaultCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, appName)
}

// jsonRequested reports whether this invocation should write a JSON
// envelope. After PersistentPreRunE, Viper holds the resolved --json
// value (flag, INCUSOS_BUILDER_JSON, or default). Flag-parse failures
// run before Viper is initialized, so a parsed --json flag is also
// honored.
func jsonRequested(cmd *cobra.Command, opts Options) bool {
	if opts.Viper != nil && opts.Viper.GetBool(flagJSON) {
		return true
	}
	if cmd == nil {
		return false
	}
	flag := cmd.Flags().Lookup(flagJSON)
	if flag == nil || !flag.Changed {
		return false
	}
	value, err := cmd.Flags().GetBool(flagJSON)
	return err == nil && value
}

// flagUsageError wraps cobra/pflag parse failures as [ErrUsage] so they
// share the exit-2 path with publisher usage errors.
func flagUsageError(_ *cobra.Command, err error) error {
	if err == nil || IsUsage(err) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrUsage, err)
}

// noArgs rejects positional operands as [ErrUsage] (exit 2). Cobra's own
// NoArgs returns an untyped error that would otherwise map to exit 1.
func noArgs(opts Options) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		var err error
		if cmd.Parent() == nil {
			err = usagef("unknown command %q for %q", args[0], cmd.CommandPath())
		} else {
			err = usagef("%s accepts no arguments", cmd.CommandPath())
		}
		return reportBuildError(opts, jsonRequested(cmd, opts), err)
	}
}
