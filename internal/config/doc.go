// Package config loads an incusos-builder YAML document into a [build.Spec].
//
// The pipeline is: read (file or stdin) → detect a top-level sops key →
// in-memory SOPS decrypt when present → strict YAML decode → validate.
// Decrypted bytes never touch the filesystem. Validation errors wrap
// [ErrConfig] (exit 3) and name field paths, never secret values.
// Decrypt failures after sops detection wrap [ErrDecrypt] (exit 4).
package config
