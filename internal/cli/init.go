package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
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
	// envAccessible enables the line-oriented prompt path.
	envAccessible = "ACCESSIBLE"
	// envTerm is the terminal-type environment variable Huh also consults.
	envTerm = "TERM"
	// termDumb is Huh's default accessible trigger.
	termDumb = "dumb"
	// schemaVersionLiteral is the only config schema version this CLI writes.
	schemaVersionLiteral = 1
	// initFileMode is the mode used for O_CREAT|O_EXCL config claims.
	initFileMode = 0o644
	// initWrote is the human success line after writing a config file.
	initWrote = "wrote %s"
	// defaultInitApplication is the offline application name when the user
	// accepts the Application name default.
	defaultInitApplication = "incus"
	// initCancelledMsg is the usage error for abort, EOF, and cancellation.
	initCancelledMsg = "init cancelled"
	// initTitleType is shown in both TUI and accessible Image type prompts.
	initTitleType = "Image type"
	// initTitleArch is shown in both TUI and accessible Architecture prompts.
	initTitleArch = "Architecture"
	// initTitleChannel is shown in both TUI and accessible Channel prompts.
	initTitleChannel = "Channel"
	// initTitleOffline is shown in both TUI and accessible Offline prompts.
	initTitleOffline = "Offline install?"
	// initTitleApplication is shown in both TUI and accessible Application prompts.
	initTitleApplication = "Application name"
	// initDescChannel is shown in both TUI and accessible Channel prompts.
	initDescChannel = "Update-server channel; default stable."
	// initDescOffline is shown in both TUI and accessible Offline prompts.
	initDescOffline = "Also build RESCUE_DATA resources media."
	// initDescApplication is shown in both TUI and accessible Application prompts.
	initDescApplication = "Offline application to include in resources media; default incus."
	// initSelectTypeISO is the accessible/TUI label for iso.
	initSelectTypeISO = "ISO installer"
	// initSelectTypeRaw is the accessible/TUI label for raw.
	initSelectTypeRaw = "Raw disk image"
	// initSelectArchX86 is the accessible/TUI label for x86_64.
	initSelectArchX86 = "x86_64"
	// initSelectArchARM is the accessible/TUI label for aarch64.
	initSelectArchARM = "aarch64"
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
	// Application is the seeds.applications name used when Offline is true.
	Application string
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
		Args:  noArgs(opts),
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
	if pol.JSON && isStdout(output) {
		return finishInit(opts, pol, usagef("--json cannot be combined with -o -"))
	}
	answers, err := collectInitAnswers(cmd.Context(), pol, opts)
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
	if pol.Quiet || isStdout(output) {
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

// collectInitAnswers runs the form when input is allowed; otherwise it
// returns the deterministic example answers (iso / x86_64 / stable / online).
func collectInitAnswers(ctx context.Context, pol policy, opts Options) (initAnswers, error) {
	if pol.NoInput {
		return exampleInitAnswers(), nil
	}
	return runInitForm(ctx, opts)
}

// exampleInitAnswers is the uncommented image block of the --no-input example.
func exampleInitAnswers() initAnswers {
	return initAnswers{
		Type:         build.ImageTypeISO,
		Architecture: build.ArchX8664,
		Channel:      build.DefaultChannel,
		Offline:      false,
		Application:  defaultInitApplication,
	}
}

// finalizeInitAnswers applies Channel and Application defaults after a form run.
func finalizeInitAnswers(answers initAnswers, channel string) initAnswers {
	answers.Channel = build.Channel(strings.TrimSpace(channel))
	if answers.Channel == "" {
		answers.Channel = build.DefaultChannel
	}
	answers.Application = strings.TrimSpace(answers.Application)
	if answers.Application == "" {
		answers.Application = defaultInitApplication
	}
	return answers
}

// runInitForm prompts for image type, architecture, channel, offline, and
// (when offline) application name.
//
// A non-empty ACCESSIBLE or TERM=dumb selects the project-owned
// line-oriented prompts. Huh's accessible runner is not used: it discards
// the context and treats EOF as a default.
func runInitForm(ctx context.Context, opts Options) (initAnswers, error) {
	if accessibleInitEnabled() {
		answers, err := runAccessibleInitForm(ctx, opts)
		if err != nil {
			return initAnswers{}, wrapInitFormError(ctx, err)
		}
		return answers, nil
	}
	answers := exampleInitAnswers()
	channel := string(answers.Channel)
	form := newInitForm(&answers, &channel).
		WithAccessible(false).
		WithTheme(huh.ThemeFunc(initTheme)).
		WithInput(opts.In).
		WithOutput(opts.Err)
	if err := form.RunWithContext(ctx); err != nil {
		return initAnswers{}, wrapInitFormError(ctx, err)
	}
	return finalizeInitAnswers(answers, channel), nil
}

// accessibleInitEnabled reports whether line-oriented prompts should run.
func accessibleInitEnabled() bool {
	return os.Getenv(envAccessible) != "" || os.Getenv(envTerm) == termDumb
}

// wrapInitFormError maps abort, EOF, and context cancellation to a usage
// error. Other failures pass through.
func wrapInitFormError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		ctx.Err() != nil {
		return usagef(initCancelledMsg)
	}
	return err
}

// newInitForm builds the Huh TUI bound to answers and channel. It does not run it.
func newInitForm(answers *initAnswers, channel *string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[build.ImageType]().
				Title(initTitleType).
				Options(
					huh.NewOption(initSelectTypeISO, build.ImageTypeISO),
					huh.NewOption(initSelectTypeRaw, build.ImageTypeRaw),
				).
				Value(&answers.Type),
			huh.NewSelect[build.Architecture]().
				Title(initTitleArch).
				Options(
					huh.NewOption(initSelectArchX86, build.ArchX8664),
					huh.NewOption(initSelectArchARM, build.ArchAarch64),
				).
				Value(&answers.Architecture),
			huh.NewInput().
				Title(initTitleChannel).
				Description(initDescChannel).
				Value(channel),
			huh.NewConfirm().
				Title(initTitleOffline).
				Description(initDescOffline).
				Affirmative("yes").
				Negative("no").
				Value(&answers.Offline),
		).Title("incusos-builder init"),
		huh.NewGroup(
			huh.NewInput().
				Title(initTitleApplication).
				Description(initDescApplication).
				Value(&answers.Application),
		).WithHideFunc(func() bool { return !answers.Offline }),
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

// runAccessibleInitForm collects answers with line prompts that honour
// context cancellation and EOF.
func runAccessibleInitForm(ctx context.Context, opts Options) (initAnswers, error) {
	answers := exampleInitAnswers()
	prompt := newInitLinePrompt(ctx, opts.In, opts.Err)
	typeIdx, err := prompt.selectOption(
		initTitleType, "",
		[]string{initSelectTypeISO, initSelectTypeRaw}, 0,
	)
	if err != nil {
		return initAnswers{}, err
	}
	answers.Type = []build.ImageType{build.ImageTypeISO, build.ImageTypeRaw}[typeIdx]
	archIdx, err := prompt.selectOption(
		initTitleArch, "",
		[]string{initSelectArchX86, initSelectArchARM}, 0,
	)
	if err != nil {
		return initAnswers{}, err
	}
	answers.Architecture = []build.Architecture{build.ArchX8664, build.ArchAarch64}[archIdx]
	channel, err := prompt.text(initTitleChannel, initDescChannel, string(build.DefaultChannel))
	if err != nil {
		return initAnswers{}, err
	}
	offline, err := prompt.confirm(initTitleOffline, initDescOffline, false)
	if err != nil {
		return initAnswers{}, err
	}
	answers.Offline = offline
	if answers.Offline {
		application, appErr := prompt.text(
			initTitleApplication, initDescApplication, defaultInitApplication,
		)
		if appErr != nil {
			return initAnswers{}, appErr
		}
		answers.Application = application
	}
	return finalizeInitAnswers(answers, channel), nil
}

// initLinePrompt is the project-owned accessible reader.
type initLinePrompt struct {
	// ctx cancels a blocked line read.
	ctx context.Context
	// lines receives stdin lines from the background scanner.
	lines <-chan string
	// errs receives the scanner's terminal error (EOF or read failure).
	errs <-chan error
	// out is the prompt writer (stderr).
	out io.Writer
}

// newInitLinePrompt starts a background scanner that can be interrupted by ctx.
func newInitLinePrompt(ctx context.Context, in io.Reader, out io.Writer) *initLinePrompt {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stderr
	}
	lines := make(chan string)
	errs := make(chan error, 1)
	go scanInitLines(ctx, in, lines, errs)
	return &initLinePrompt{ctx: ctx, lines: lines, errs: errs, out: out}
}

// scanInitLines sends newline-delimited input until ctx is done or the reader ends.
func scanInitLines(ctx context.Context, in io.Reader, lines chan<- string, errs chan<- error) {
	defer close(lines)
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		case lines <- scanner.Text():
		}
	}
	if err := scanner.Err(); err != nil {
		errs <- fmt.Errorf("read init input: %w", err)
		return
	}
	errs <- io.EOF
}

// line reads the next input line or returns a cancellation/EOF error.
func (p *initLinePrompt) line() (string, error) {
	select {
	case <-p.ctx.Done():
		return "", p.ctx.Err()
	case err := <-p.errs:
		return "", err
	case text, ok := <-p.lines:
		if !ok {
			return "", io.EOF
		}
		return text, nil
	}
}

// selectOption prints title, optional description, numbered options, and a defaulted prompt.
func (p *initLinePrompt) selectOption(title, description string, options []string, defaultIdx int) (int, error) {
	p.printLine(title)
	if description != "" {
		p.printLine(description)
	}
	for i, option := range options {
		_, _ = fmt.Fprintf(p.out, "%d. %s\n", i+1, option)
	}
	low, high := 1, len(options)
	for {
		_, _ = fmt.Fprintf(p.out, "Enter a number between %d and %d: ", low, high)
		text, err := p.line()
		if err != nil {
			return 0, err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return defaultIdx, nil
		}
		choice, convErr := strconv.Atoi(text)
		if convErr != nil || choice < low || choice > high {
			_, _ = fmt.Fprintf(p.out, "Invalid: must be a number between %d and %d\n", low, high)
			continue
		}
		return choice - 1, nil
	}
}

// text prints title, description, and a defaulted string prompt.
func (p *initLinePrompt) text(title, description, defaultValue string) (string, error) {
	p.printLine(title)
	if description != "" {
		p.printLine(description)
	}
	_, _ = fmt.Fprintf(p.out, "%s [%s]: ", title, defaultValue)
	text, err := p.line()
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultValue, nil
	}
	return text, nil
}

// confirm prints title, description, and a yes/no prompt with a default.
func (p *initLinePrompt) confirm(title, description string, defaultValue bool) (bool, error) {
	p.printLine(title)
	if description != "" {
		p.printLine(description)
	}
	hint := "[y/N]"
	if defaultValue {
		hint = "[Y/n]"
	}
	for {
		_, _ = fmt.Fprintf(p.out, "%s %s: ", title, hint)
		text, err := p.line()
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "":
			return defaultValue, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			p.printLine("invalid input. please try again")
		}
	}
}

// printLine writes s plus a newline to the prompt writer.
func (p *initLinePrompt) printLine(s string) {
	_, _ = fmt.Fprintln(p.out, s)
}

// renderInitConfig writes a YAML document whose uncommented body is a valid
// config.Parse input. Under noInput the seeds section is listed as comments
// from [seedSections] so it cannot drift from [build.Seeds]. Interactive
// offline answers emit seeds.applications.applications from Application.
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
		return b.String()
	}
	if answers.Offline {
		writeOfflineApplicationSeed(&b, answers.Application)
	}
	return b.String()
}

// writeOfflineApplicationSeed renders seeds.applications.applications from name.
func writeOfflineApplicationSeed(b *strings.Builder, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultInitApplication
	}
	b.WriteString("seeds:\n")
	b.WriteString("  applications:\n")
	b.WriteString("    applications:\n")
	fmt.Fprintf(b, "      - name: %s\n", strconv.Quote(name))
}

// writeInitOutput writes body to path, or to stdout when path is the "-"
// sentinel (including cleaned forms such as ./-). An existing path is a
// usage error; init has no --force.
func writeInitOutput(path, body string, stdout io.Writer) error {
	if isStdout(path) {
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
