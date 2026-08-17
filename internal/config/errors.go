package config

import "github.com/componere/incusos-builder/internal/errdefs"

// ErrConfig is returned when the configuration document is invalid.
// Callers map it to process exit code 3. It is the package-local name for
// [errdefs.ErrConfig]; [errors.Is] matches through either.
var ErrConfig = errdefs.ErrConfig

// ErrDecrypt is returned when SOPS decryption fails after a top-level
// sops key was detected. Every failure on the encrypted path wraps it;
// none fall through to a decode error. Callers map it to process exit
// code 4. It is the package-local name for [errdefs.ErrDecrypt].
var ErrDecrypt = errdefs.ErrDecrypt
