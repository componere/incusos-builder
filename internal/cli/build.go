package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/pgzip"
	"github.com/spf13/cobra"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/config"
	"github.com/componere/incusos-builder/internal/errdefs"
	"github.com/componere/incusos-builder/internal/media"
	"github.com/componere/incusos-builder/internal/seed"
	"github.com/componere/incusos-builder/internal/update"
	"github.com/componere/incusos-builder/internal/ux"
)

const (
	// flagOutput is the -o/--output flag shared by build.
	flagOutput = "output"
	// flagResourcesOutput is the --resources-output flag.
	flagResourcesOutput = "resources-output"
	// flagForce is the --force overwrite flag.
	flagForce = "force"
	// httpsPrefix is the scheme that selects the HTTPS image source.
	httpsPrefix = "https://"
	// httpPrefix is rejected by the HTTPS adapter (ARCHITECTURE §6).
	httpPrefix = "http://"
	// gzipSuffix selects post-splice recompression of the stored image.
	gzipSuffix = ".gz"
	// confirmPrompt is the y/N overwrite question written to stderr.
	confirmPrompt = "overwrite existing output? [y/N] "
)

// buildEnvelope is the --json success document from ARCHITECTURE §3.
type buildEnvelope struct {
	// Result is the success body.
	Result buildResult `json:"result"`
}

// buildResult names the published artifacts and the resolved image.
type buildResult struct {
	// Output is the -o path, or "-" when the image was streamed to stdout.
	Output string `json:"output"`
	// ResourcesOutput is the --resources-output path. Empty when online.
	ResourcesOutput string `json:"resources_output,omitempty"`
	// Type is the image type (iso or raw).
	Type string `json:"type"`
	// Architecture is the CPU architecture.
	Architecture string `json:"architecture"`
	// Version is the resolved update version.
	Version string `json:"version"`
	// Channel is the channel the version was selected from.
	Channel string `json:"channel"`
	// SeedBytes is the spliced seed-tar size.
	SeedBytes int64 `json:"seed_bytes"`
	// SHA256 is the lowercase hex digest of the stored image bytes.
	SHA256 string `json:"sha256"`
	// ResourcesSHA256 is the lowercase hex digest of the resources media.
	// Empty when online.
	ResourcesSHA256 string `json:"resources_sha256,omitempty"`
}

// buildDeps are the collaborators [runBuildCommand] uses after flags resolve.
// Tests replace individual fields; production leaves them nil so the
// real adapters and [seed.Render] are constructed in RunE.
type buildDeps struct {
	// Load returns a validated spec from -f (or stdin when path is "-").
	Load func(path string, stdin io.Reader) (build.Spec, error)
	// Source constructs the ImageSource for --server.
	Source func(server, cacheDir string, reporter build.Reporter) (build.ImageSource, error)
	// Rescue constructs the RescueWriter.
	Rescue func() build.RescueWriter
	// Reporter constructs the progress reporter written to stderr.
	Reporter func(color ux.ColorMode, progress ux.ProgressMode, w io.Writer) build.Reporter
	// Render serializes seeds into a tar. Nil uses [seed.Render].
	Render build.SeedRenderFunc
	// Confirm, when set, is passed to [Begin] instead of the y/N prompt.
	// [policy.NoInput] still forces a nil Confirm so the publisher never blocks.
	Confirm ConfirmFunc
}

// newBuildCommand constructs the build subcommand. It is not registered
// here; the orchestrator wires AddCommand.
func newBuildCommand(opts Options) *cobra.Command {
	return newBuildCommandWith(opts, buildDeps{})
}

// newBuildCommandWith constructs the build subcommand with injected
// collaborators. Production uses [newBuildCommand].
func newBuildCommandWith(opts Options, deps buildDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build seeded IncusOS installation media from a YAML config",
		Long:  "Build seeded IncusOS installation media from a YAML config. -f - reads the config from stdin; -o - writes the image to stdout.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBuildCommand(cmd, opts, deps)
		},
	}
	cmd.Flags().StringP(flagConfig, "f", "", "path to config YAML (`-` reads stdin)")
	cmd.Flags().StringP(flagOutput, "o", "", "image output path (`-` writes stdout)")
	cmd.Flags().String(flagResourcesOutput, "", "resources-media output path (offline builds)")
	cmd.Flags().Bool(flagForce, false, "replace existing output files")
	return cmd
}

// runBuildCommand resolves policy, validates usage, wires adapters, and
// either streams the image to stdout or publishes files through [Begin].
func runBuildCommand(cmd *cobra.Command, opts Options, deps buildDeps) error {
	pol, err := resolvePolicy(cmd, opts, opts.Viper)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	configPath, err := cmd.Flags().GetString(flagConfig)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	output, err := cmd.Flags().GetString(flagOutput)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	resources, err := cmd.Flags().GetString(flagResourcesOutput)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	force, err := cmd.Flags().GetBool(flagForce)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	if err := checkBuildFlags(configPath, output, pol.JSON); err != nil {
		return finishCommand(opts, pol, err)
	}

	spec, err := loadBuildSpec(deps, configPath, opts.In)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	if err := checkBuildSpecUsage(output, spec); err != nil {
		return finishCommand(opts, pol, err)
	}

	toStdout := isStdout(output)
	color, progress := reporterModes(pol, toStdout)
	reporter := buildReporter(deps, color, progress, opts.Err)
	src, err := openImageSource(deps, pol.Server, pol.CacheDir, reporter)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	rescue := buildRescue(deps)
	render := deps.Render
	if render == nil {
		render = seed.Render
	}

	if toStdout {
		result, digest, runErr := streamBuild(cmd, opts, spec, src, rescue, reporter, render, output)
		if runErr != nil {
			return runErr
		}
		return finishBuild(opts, pol, toStdout, output, "", result, digest, "")
	}

	session, err := Begin(Request{
		Image:     output,
		Resources: resources,
		Offline:   spec.Offline,
		Type:      spec.Type,
		Force:     force,
		Confirm:   confirmFor(pol, opts, deps.Confirm),
	})
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	defer session.Abort()

	dest, closer := wrapStoredWriter(output, session.ImageWriter())
	result, err := build.Build(cmd.Context(), spec, src, rescue, reporter, render, dest, session.ResourcesTemp())
	closeErr := closer()
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	if closeErr != nil {
		return finishCommand(opts, pol, outputWrap(closeErr, "close image stream"))
	}
	pub, err := session.Publish()
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	paths := session.Paths()
	return finishBuild(opts, pol, false, paths.Image, paths.Resources, result, pub.ImageSHA256, pub.ResourcesSHA256)
}

// checkBuildFlags rejects missing required flags and --json with -o -.
func checkBuildFlags(configPath, output string, jsonMode bool) error {
	if strings.TrimSpace(configPath) == "" {
		return usagef("-f/--config is required")
	}
	if strings.TrimSpace(output) == "" {
		return usagef("-o/--output is required")
	}
	if jsonMode && isStdout(output) {
		return usagef("--json cannot be combined with -o -")
	}
	return nil
}

// checkBuildSpecUsage rejects offline builds that would share one stdout stream.
func checkBuildSpecUsage(output string, spec build.Spec) error {
	if spec.Offline && isStdout(output) {
		return usagef("offline builds cannot use -o -")
	}
	return nil
}

// loadBuildSpec reads and validates the config. Path "-" uses stdin.
func loadBuildSpec(deps buildDeps, path string, stdin io.Reader) (build.Spec, error) {
	if deps.Load != nil {
		return deps.Load(path, stdin)
	}
	if path == stdoutSentinel {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return build.Spec{}, fmt.Errorf("%w: read stdin: %w", errdefs.ErrConfig, err)
		}
		return config.Parse(raw)
	}
	return config.Load(path)
}

// openImageSource selects the HTTPS or local-directory adapter from --server.
//
// An https:// URL uses [update.NewHTTPSSource] (plain http is the adapter's
// rule and surfaces as [errdefs.ErrFetch]). An existing directory uses
// [update.NewLocalSource]. Anything else is a usage error.
func openImageSource(
	deps buildDeps,
	server, cacheDir string,
	reporter build.Reporter,
) (build.ImageSource, error) {
	if deps.Source != nil {
		return deps.Source(server, cacheDir, reporter)
	}
	return selectImageSource(server, cacheDir, reporter)
}

// selectImageSource is the production --server adapter rule: an https URL
// selects the HTTPS source, an existing directory selects the local
// source, and anything else — including plain http, which the adapter
// would also refuse — is a usage error (exit 2): a bad flag value, not an
// acquisition failure.
func selectImageSource(server, cacheDir string, reporter build.Reporter) (build.ImageSource, error) {
	lower := strings.ToLower(server)
	if strings.HasPrefix(lower, httpPrefix) && !strings.HasPrefix(lower, httpsPrefix) {
		return nil, usagef("--server %q: plain http is not supported; use https or a local mirror directory", server)
	}
	if strings.HasPrefix(lower, httpsPrefix) {
		return update.NewHTTPSSource(server, cacheDir, reporter, nil)
	}
	info, err := os.Stat(server)
	if err == nil && info.IsDir() {
		return update.NewLocalSource(server, cacheDir, reporter)
	}
	return nil, usagef("--server %q is neither an https URL nor an existing directory", server)
}

// buildReporter constructs the stderr reporter, or the injected one.
func buildReporter(
	deps buildDeps,
	color ux.ColorMode,
	progress ux.ProgressMode,
	w io.Writer,
) build.Reporter {
	if deps.Reporter != nil {
		return deps.Reporter(color, progress, w)
	}
	return ux.New(color, progress, w)
}

// buildRescue constructs the rescue-media writer, or the injected one.
func buildRescue(deps buildDeps) build.RescueWriter {
	if deps.Rescue != nil {
		return deps.Rescue()
	}
	return media.NewWriter()
}

// reporterModes suppress AUTO progress when stdout is reserved for artifact
// bytes or a JSON envelope. Explicit --progress always/never is unchanged.
func reporterModes(pol policy, toStdout bool) (ux.ColorMode, ux.ProgressMode) {
	progress := pol.Progress
	if (pol.JSON || toStdout) && progress == ux.ProgressModeAuto {
		progress = ux.ProgressModeNever
	}
	return pol.Color, progress
}

// confirmFor returns the overwrite callback passed to [Begin].
//
// Under [policy.NoInput] the publisher receives a nil Confirm and refuses
// existing finals with a usage error (exit 2) instead of prompting.
func confirmFor(pol policy, opts Options, injected ConfirmFunc) ConfirmFunc {
	if pol.NoInput {
		return nil
	}
	if injected != nil {
		return injected
	}
	return promptConfirm(opts.In, opts.Err)
}

// promptConfirm reads a y/N answer from in. The question is written to errW
// so stdout stays free for summaries and JSON.
func promptConfirm(in io.Reader, errW io.Writer) ConfirmFunc {
	return func() (bool, error) {
		if _, err := io.WriteString(errW, confirmPrompt); err != nil {
			return false, outputWrap(err, "write confirm prompt")
		}
		scanner := bufio.NewScanner(in)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return false, outputWrap(err, "read confirm answer")
			}
			return false, nil
		}
		switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
		case "y", "yes":
			return true, nil
		default:
			return false, nil
		}
	}
}

// wrapStoredWriter returns dest, or a pgzip writer in front of dest when
// path ends in .gz. The closer must be called so the gzip footer is part of
// the hashed stored bytes (ARCHITECTURE §3).
func wrapStoredWriter(path string, dest io.Writer) (io.Writer, func() error) {
	if !strings.HasSuffix(path, gzipSuffix) {
		return dest, func() error { return nil }
	}
	gz := pgzip.NewWriter(dest)
	gz.ModTime = time.Time{}
	gz.OS = 255
	return gz, gz.Close
}

// streamBuild writes the spliced image through a hashing writer to stdout.
// There is no publisher and no resources temp. Summary is the caller's job
// and is suppressed by [finishBuild] for this path.
func streamBuild(
	cmd *cobra.Command,
	opts Options,
	spec build.Spec,
	src build.ImageSource,
	rescue build.RescueWriter,
	reporter build.Reporter,
	render build.SeedRenderFunc,
	output string,
) (build.Result, string, error) {
	hasher := &hashingWriter{w: opts.Out, h: sha256.New()}
	dest, closer := wrapStoredWriter(output, hasher)
	result, err := build.Build(cmd.Context(), spec, src, rescue, reporter, render, dest, "")
	closeErr := closer()
	if err != nil {
		return build.Result{}, "", err
	}
	if closeErr != nil {
		return build.Result{}, "", outputWrap(closeErr, "close image stream")
	}
	return result, hex.EncodeToString(hasher.h.Sum(nil)), nil
}

// finishBuild writes the human summary or the --json success envelope.
// -o - never writes a summary; stdout already holds artifact bytes.
func finishBuild(
	opts Options,
	pol policy,
	toStdout bool,
	output, resources string,
	result build.Result,
	imageSHA, resourcesSHA string,
) error {
	payload := buildResult{
		Output:          output,
		ResourcesOutput: resources,
		Type:            string(result.Type),
		Architecture:    string(result.Architecture),
		Version:         result.Version,
		Channel:         string(result.Channel),
		SeedBytes:       result.SeedBytes,
		SHA256:          imageSHA,
		ResourcesSHA256: resourcesSHA,
	}
	if pol.JSON {
		return writeJSON(opts.Out, payload)
	}
	if toStdout || pol.Quiet {
		return nil
	}
	writeBuildSummary(pol.Color, opts.Out, payload)
	return nil
}

// writeBuildJSON writes exactly one {"result":{...}} document to w.
func writeBuildJSON(w io.Writer, payload buildResult) error {
	return writeJSON(w, payload)
}

// writeBuildSummary prints the human-readable result block to w.
func writeBuildSummary(color ux.ColorMode, w io.Writer, payload buildResult) {
	rows := []ux.SummaryRow{
		{Label: "output", Value: payload.Output},
	}
	if payload.ResourcesOutput != "" {
		rows = append(rows, ux.SummaryRow{Label: "resources_output", Value: payload.ResourcesOutput})
	}
	rows = append(rows,
		ux.SummaryRow{Label: "type", Value: payload.Type},
		ux.SummaryRow{Label: "architecture", Value: payload.Architecture},
		ux.SummaryRow{Label: "version", Value: payload.Version},
		ux.SummaryRow{Label: "channel", Value: payload.Channel},
		ux.SummaryRow{Label: "seed_bytes", Value: strconv.FormatInt(payload.SeedBytes, 10)},
		ux.SummaryRow{Label: "sha256", Value: payload.SHA256},
	)
	if payload.ResourcesSHA256 != "" {
		rows = append(rows, ux.SummaryRow{Label: "resources_sha256", Value: payload.ResourcesSHA256})
	}
	ux.Summary(color, w, rows)
}

// reportBuildError writes the --json error envelope to stdout when jsonMode
// is set, then returns err so [exitCode] still maps the sentinel. jsonMode
// is false after a -o - stream has started so a mid-stream failure cannot
// append a second document to the artifact.
func reportBuildError(opts Options, jsonMode bool, err error) error {
	if err == nil || !jsonMode {
		return err
	}
	if encErr := writeErrorJSON(opts.Out, err); encErr != nil {
		return err
	}
	return err
}
