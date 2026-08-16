package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/ux"
)

const (
	// cmdNameInit is the init subcommand name.
	cmdNameInit = "init"
	// flagInitOutput is the -o/--output flag for init.
	flagInitOutput = "output"
	// defaultInitOutput is the default path written by init.
	defaultInitOutput = "config.yaml"
	// envAccessible enables Huh's screen-reader prompt mode.
	envAccessible = "ACCESSIBLE"
	// schemaVersionLiteral is the only config schema version this CLI writes.
	schemaVersionLiteral = 1
	// initFileMode is the mode used for O_CREAT|O_EXCL config claims.
	initFileMode = 0o644
	// initWrote is the human success line after writing a config file.
	initWrote = "wrote %s"
)

// seedSection is one commented seeds. key in the --no-input example.
type seedSection struct {
	// Field is the exported [build.Seeds] field name.
	Field string
	// YAML is the config-file key (kebab-case where the schema requires it).
	YAML string
}

// seedSections lists every [build.Seeds] field in struct order. YAML names
// match internal/config's schema tags. Tests assert this table against the
// live struct so a new section cannot ship without appearing in the example.
func seedSections() []seedSection {
	return []seedSection{
		{Field: "Applications", YAML: "applications"},
		{Field: "Incus", YAML: "incus"},
		{Field: "Install", YAML: "install"},
		{Field: "MigrationManager", YAML: "migration-manager"},
		{Field: "Network", YAML: "network"},
		{Field: "OperationsCenter", YAML: "operations-center"},
		{Field: "Provider", YAML: "provider"},
		{Field: "Services", YAML: "services"},
		{Field: "Update", YAML: "update"},
		{Field: "Kernel", YAML: "kernel"},
		{Field: "Security", YAML: "security"},
	}
}

// initAnswers is the image block collected by the interactive form or
// supplied as the uncommented body of the --no-input example.
type initAnswers struct {
	// Type is iso or raw.
	Type build.ImageType
	// Architecture is x86_64 or aarch64.
	Architecture build.Architecture
	// Channel is the update-server channel; empty becomes stable at parse time.
	Channel build.Channel
	// Offline also builds RESCUE_DATA resources media.
	Offline bool
}

// initEnvelope is the --json success document for init.
type initEnvelope struct {
	// Result is the success body.
	Result initResult `json:"result"`
}

// initResult names the file that was written.
type initResult struct {
	// Output is the -o path.
	Output string `json:"output"`
}

// newInitCommand constructs the init subcommand. It is not registered here;
// the orchestrator wires AddCommand.
func newInitCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdNameInit,
		Short: "Write a starter config.yaml",
		Long:  "Write a starter config.yaml. Interactive mode collects image settings; --no-input writes a commented example generated from the schema.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, opts)
		},
	}
	cmd.Flags().StringP(flagInitOutput, "o", defaultInitOutput, "output path (`-` writes stdout)")
	return cmd
}

// runInit resolves policy, collects answers, and writes the config.
func runInit(cmd *cobra.Command, opts Options) error {
	pol, err := resolvePolicy(cmd, opts, opts.Viper)
	if err != nil {
		return finishInit(opts, pol, err)
	}
	output, err := cmd.Flags().GetString(flagInitOutput)
	if err != nil {
		return finishInit(opts, pol, err)
	}
	if pol.JSON && output == stdoutSentinel {
		return finishInit(opts, pol, usagef("--json cannot be combined with -o -"))
	}
	answers, err := collectInitAnswers(pol, opts)
	if err != nil {
		return finishInit(opts, pol, err)
	}
	body := renderInitConfig(answers, pol.NoInput)
	if err = writeInitOutput(output, body, opts.Out); err != nil {
		return finishInit(opts, pol, err)
	}
	if pol.JSON {
		enc := json.NewEncoder(opts.Out)
		return enc.Encode(initEnvelope{Result: initResult{Output: output}})
	}
	if pol.Quiet || output == stdoutSentinel {
		return nil
	}
	_, err = fmt.Fprintf(opts.Out, initWrote+"\n", output)
	return err
}

// finishInit writes a JSON error envelope when --json is set, then returns err.
func finishInit(opts Options, pol policy, err error) error {
	if err == nil {
		return nil
	}
	if pol.JSON || initJSONRequested(opts) {
		_ = writeErrorJSON(opts.Out, err)
	}
	return err
}

// initJSONRequested reports whether Viper has --json after PersistentPreRunE.
func initJSONRequested(opts Options) bool {
	return opts.Viper != nil && opts.Viper.GetBool(flagJSON)
}

// collectInitAnswers runs the Huh form when input is allowed; otherwise it
// returns the deterministic example answers (iso / x86_64 / stable / online).
func collectInitAnswers(pol policy, opts Options) (initAnswers, error) {
	if pol.NoInput {
		return exampleInitAnswers(), nil
	}
	return runInitForm(opts)
}

// exampleInitAnswers is the uncommented image block of the --no-input example.
func exampleInitAnswers() initAnswers {
	return initAnswers{
		Type:         build.ImageTypeISO,
		Architecture: build.ArchX8664,
		Channel:      build.DefaultChannel,
		Offline:      false,
	}
}

// runInitForm prompts for image type, architecture, channel, and offline.
//
// ACCESSIBLE (any non-empty value) and TERM=dumb (Huh's own default) select
// the line-oriented prompt path. Form construction is cheap to test; the
// accessible path can be scripted via [huh.Form.WithInput]. Full-screen TTY
// driving is left to a later smoke test.
func runInitForm(opts Options) (initAnswers, error) {
	answers := exampleInitAnswers()
	channel := string(answers.Channel)
	form := newInitForm(&answers, &channel).
		WithAccessible(os.Getenv(envAccessible) != "").
		WithTheme(huh.ThemeFunc(initTheme)).
		WithInput(opts.In).
		WithOutput(opts.Err)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return initAnswers{}, usagef("init cancelled")
		}
		return initAnswers{}, err
	}
	answers.Channel = build.Channel(strings.TrimSpace(channel))
	if answers.Channel == "" {
		answers.Channel = build.DefaultChannel
	}
	return answers, nil
}

// newInitForm builds the Huh form bound to answers and channel. It does not run it.
func newInitForm(answers *initAnswers, channel *string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[build.ImageType]().
				Title("Image type").
				Options(
					huh.NewOption("ISO installer", build.ImageTypeISO),
					huh.NewOption("Raw disk image", build.ImageTypeRaw),
				).
				Value(&answers.Type),
			huh.NewSelect[build.Architecture]().
				Title("Architecture").
				Options(
					huh.NewOption("x86_64", build.ArchX8664),
					huh.NewOption("aarch64", build.ArchAarch64),
				).
				Value(&answers.Architecture),
			huh.NewInput().
				Title("Channel").
				Description("Update-server channel; default stable.").
				Placeholder(string(build.DefaultChannel)).
				Value(channel),
			huh.NewConfirm().
				Title("Offline install?").
				Description("Also build RESCUE_DATA resources media.").
				Affirmative("yes").
				Negative("no").
				Value(&answers.Offline),
		).Title("incusos-builder init"),
	)
}

// initTheme paints Huh from the shared ux palette so prompts match summaries.
func initTheme(isDark bool) *huh.Styles {
	styles := huh.ThemeBase(isDark)
	p := ux.DefaultPalette()
	styles.Focused.Base = styles.Focused.Base.BorderForeground(p.Subtle)
	styles.Focused.Title = styles.Focused.Title.Foreground(p.Accent).Bold(true)
	styles.Focused.NoteTitle = styles.Focused.NoteTitle.Foreground(p.Accent).Bold(true)
	styles.Focused.Description = styles.Focused.Description.Foreground(p.Subtle)
	styles.Focused.ErrorIndicator = styles.Focused.ErrorIndicator.Foreground(p.Error)
	styles.Focused.ErrorMessage = styles.Focused.ErrorMessage.Foreground(p.Error)
	styles.Focused.SelectSelector = styles.Focused.SelectSelector.Foreground(p.Accent)
	styles.Focused.NextIndicator = styles.Focused.NextIndicator.Foreground(p.Accent)
	styles.Focused.PrevIndicator = styles.Focused.PrevIndicator.Foreground(p.Accent)
	styles.Focused.Option = styles.Focused.Option.Foreground(lipgloss.NoColor{})
	styles.Focused.SelectedOption = styles.Focused.SelectedOption.Foreground(p.Success)
	styles.Focused.FocusedButton = styles.Focused.FocusedButton.Foreground(p.Success).Background(p.Accent)
	styles.Focused.BlurredButton = styles.Focused.BlurredButton.Foreground(p.Subtle)
	styles.Focused.TextInput.Cursor = styles.Focused.TextInput.Cursor.Foreground(p.Success)
	styles.Focused.TextInput.Placeholder = styles.Focused.TextInput.Placeholder.Foreground(p.Subtle)
	styles.Focused.TextInput.Prompt = styles.Focused.TextInput.Prompt.Foreground(p.Accent)
	styles.Blurred = styles.Focused
	styles.Blurred.Base = styles.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	styles.Blurred.Card = styles.Blurred.Base
	styles.Group.Title = styles.Focused.Title
	styles.Group.Description = styles.Focused.Description
	return styles
}

// renderInitConfig writes a YAML document whose uncommented body is a valid
// config.Parse input. Under noInput the seeds section is listed as comments
// from [seedSections] so it cannot drift from [build.Seeds].
func renderInitConfig(answers initAnswers, noInput bool) string {
	var b strings.Builder
	if noInput {
		b.WriteString("# Generated by incusos-builder init --no-input.\n")
		b.WriteString("# Uncommented keys are a valid config; commented seeds list every\n")
		b.WriteString("# section accepted by this CLI (from build.Seeds YAML names).\n")
		b.WriteString("#\n")
		b.WriteString("# image.type: iso | raw\n")
		b.WriteString("# image.architecture: x86_64 | aarch64\n")
		b.WriteString("# image.channel: free text; omitted defaults to stable\n")
		b.WriteString("# image.release: optional exact version pin\n")
		b.WriteString("# image.offline: true also builds RESCUE_DATA resources media\n")
		b.WriteString("#\n")
	}
	fmt.Fprintf(&b, "version: %d\n", schemaVersionLiteral)
	b.WriteString("image:\n")
	fmt.Fprintf(&b, "  type: %s\n", answers.Type)
	fmt.Fprintf(&b, "  architecture: %s\n", answers.Architecture)
	if answers.Channel != "" {
		fmt.Fprintf(&b, "  channel: %s\n", answers.Channel)
	}
	fmt.Fprintf(&b, "  offline: %t\n", answers.Offline)
	if noInput {
		b.WriteString("#seeds:\n")
		for _, section := range seedSections() {
			fmt.Fprintf(&b, "#  %s: {}\n", section.YAML)
		}
	}
	return b.String()
}

// writeInitOutput writes body to path, or to stdout when path is "-".
// An existing path is a usage error; init has no --force.
func writeInitOutput(path, body string, stdout io.Writer) error {
	if path == stdoutSentinel {
		if _, err := io.WriteString(stdout, body); err != nil {
			return fmt.Errorf("write config to stdout: %w", err)
		}
		return nil
	}
	if path == "" {
		return usagef("output path is required")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, initFileMode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return usagef("refusing to overwrite existing file %s", path)
		}
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer file.Close()
	if _, err := io.WriteString(file, body); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
