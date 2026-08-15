package config

import "errors"

// ErrConfig is returned when the configuration document is invalid.
// Callers map it to process exit code 3.
var ErrConfig = errors.New("invalid config") //nolint:gochecknoglobals // E1 sentinel for errors.Is

// ErrDecrypt is returned when SOPS decryption fails after a top-level
// sops key was detected. Callers map it to process exit code 4.
// Every failure on the encrypted path wraps ErrDecrypt; none fall
// through to a decode error.
var ErrDecrypt = errors.New("decryption failed") //nolint:gochecknoglobals // E1 sentinel for errors.Is
