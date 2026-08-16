package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	// flagConfig is the -f/--config flag shared by validate and build.
	flagConfig = "config"
	// validateOK is the human success line for validate.
	validateOK = "configuration valid"
)

// validateResult is the --json success body for validate.
type validateResult struct {
	// Valid is always true on the success path.
	Valid bool `json:"valid"`
	// Type is the image type from the config.
	Type build.ImageType `json:"type"`
	// Architecture is the image architecture from the config.
	Architecture build.Architecture `json:"architecture"`
	// Offline is the offline flag from the config.
	Offline bool `json:"offline"`
}

// newValidateCommand returns the validate subcommand.
func newValidateCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a build configuration without fetching images",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runValidate(cmd, opts)
		},
	}
	cmd.Flags().StringP(flagConfig, "f", "", "path to config file, or - for stdin (required)")
	return cmd
}

// runValidate loads and validates the config named by -f. It performs no
// network I/O. Failures propagate the config/decrypt sentinels; --json
// writes the error envelope before returning.
func runValidate(cmd *cobra.Command, opts Options) error {
	pol, err := resolvePolicy(cmd, opts, opts.Viper)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	path, err := cmd.Flags().GetString(flagConfig)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	if path == "" {
		return finishCommand(opts, pol, usagef("-f/--config is required"))
	}
	spec, err := loadBuildSpec(buildDeps{}, path, opts.In)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	if pol.JSON {
		return writeJSON(opts.Out, validateResult{
			Valid:        true,
			Type:         spec.Type,
			Architecture: spec.Architecture,
			Offline:      spec.Offline,
		})
	}
	if pol.Quiet {
		return nil
	}
	_, err = fmt.Fprintln(opts.Out, validateOK)
	return err
}

// finishCommand writes the --json error envelope when requested, then
// returns err so the process exit code still maps from the sentinel.
func finishCommand(opts Options, pol policy, err error) error {
	return reportBuildError(opts, pol.JSON, err)
}

// writeJSON writes a single {"result":...} document to w.
func writeJSON(w io.Writer, result any) error {
	payload := struct {
		// Result is the success body.
		Result any `json:"result"`
	}{Result: result}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return fmt.Errorf("encode result envelope: %w", err)
	}
	return nil
}
