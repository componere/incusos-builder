package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/pflag"

	"github.com/componere/incusos-builder/internal/errdefs"
)

const (
	// exitSuccess is process exit 0.
	exitSuccess = 0
	// exitInternal is process exit 1 (unexpected).
	exitInternal = 1
	// exitUsage is process exit 2 (usage / flag parse).
	exitUsage = 2
	// exitConfig is process exit 3 ([errdefs.ErrConfig]).
	exitConfig = 3
	// exitDecrypt is process exit 4 ([errdefs.ErrDecrypt]).
	exitDecrypt = 4
	// exitFetch is process exit 5 ([errdefs.ErrFetch] / [errdefs.ErrVersionNotFound]).
	exitFetch = 5
	// exitOutput is process exit 6 ([errdefs.ErrOutput]).
	exitOutput = 6
)

// errorEnvelope is the --json failure document.
type errorEnvelope struct {
	// Error is the code and message written under the "error" key.
	Error errorBody `json:"error"`
}

// errorBody is the code/message pair inside [errorEnvelope].
type errorBody struct {
	// Code is the process exit code that accompanies the envelope.
	Code int `json:"code"`
	// Message is err.Error(); empty when err is nil.
	Message string `json:"message"`
}

// exitCode maps err to the process exit code.
func exitCode(err error) int {
	if err == nil {
		return exitSuccess
	}
	switch {
	case IsUsage(err), isFlagError(err), isUnknownCommandError(err):
		return exitUsage
	case errors.Is(err, errdefs.ErrConfig):
		return exitConfig
	case errors.Is(err, errdefs.ErrDecrypt):
		return exitDecrypt
	case errors.Is(err, errdefs.ErrFetch), errors.Is(err, errdefs.ErrVersionNotFound):
		return exitFetch
	case errors.Is(err, errdefs.ErrOutput):
		return exitOutput
	default:
		return exitInternal
	}
}

// isFlagError reports whether err is a cobra/pflag parse failure that did
// not wrap [ErrUsage].
func isFlagError(err error) bool {
	var notExist *pflag.NotExistError
	var valueRequired *pflag.ValueRequiredError
	var invalidValue *pflag.InvalidValueError
	var invalidSyntax *pflag.InvalidSyntaxError
	return errors.As(err, &notExist) ||
		errors.As(err, &valueRequired) ||
		errors.As(err, &invalidValue) ||
		errors.As(err, &invalidSyntax)
}

// isUnknownCommandError reports whether err is Cobra's untyped
// `unknown command "..." for "..."` failure (legacyArgs / NoArgs). Those
// strings are usage errors (exit 2) but do not wrap [ErrUsage].
func isUnknownCommandError(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(err.Error(), "unknown command ")
}

// writeErrorJSON writes {"error":{code,message}} for --json failure paths.
func writeErrorJSON(w io.Writer, err error) error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	payload := errorEnvelope{
		Error: errorBody{
			Code:    exitCode(err),
			Message: msg,
		},
	}
	enc := json.NewEncoder(w)
	if encErr := enc.Encode(payload); encErr != nil {
		return fmt.Errorf("encode error envelope: %w", encErr)
	}
	return nil
}
