// Package cli is the incusos-builder command-line surface: the Cobra root,
// interaction policy, sentinel-to-exit-code mapping, and the output publisher.
//
// [NewRootCommand] constructs the root command. Global flags and Viper
// precedence (flags > INCUSOS_BUILDER_* > defaults) are applied in
// PersistentPreRunE after flags are parsed. [Execute] runs that command and
// returns the process exit code. [IncusOSPin] reads the linked incus-osd
// module version for the second --version line. [ErrUsage] and the [errdefs]
// sentinels are mapped to process exit codes in one place.
package cli
