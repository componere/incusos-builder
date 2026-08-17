// Package cli is the incusos-builder command-line surface: the Cobra root,
// interaction policy, sentinel-to-exit-code mapping, and the output publisher.
//
// [NewRootCommand] constructs the root command. Global flags and Viper
// precedence (flags > INCUSOS_BUILDER_* > defaults) are applied in
// PersistentPreRunE after flags are parsed. [Execute] runs that command and
// returns the process exit code. [IncusOSPin] reads the linked incus-osd
// module version for the second --version line.
//
// Process exit codes:
//
//	0 success
//	1 unexpected error
//	2 usage or flag-parse failure ([ErrUsage])
//	3 invalid configuration ([errdefs.ErrConfig])
//	4 decrypt failure ([errdefs.ErrDecrypt])
//	5 fetch or missing version ([errdefs.ErrFetch], [errdefs.ErrVersionNotFound])
//	6 output failure ([errdefs.ErrOutput])
package cli
