// Package seed renders the IncusOS seed-data tar byte-compatibly with
// upstream image-customizer writeSeed.
//
// Each non-nil [build.Seeds] section becomes one uncompressed tar entry of
// YAML produced by yaml.Dump(..., yaml.WithV2Defaults()). The nine web
// customizer sections use writeSeed's entry names and order. Kernel and
// security follow as kernel.yaml and security.yaml, the filenames
// incus-osd reads. Headers set Name, Mode 0600, and Size only. Closing the
// tar writer supplies the trailing end-of-archive blocks included in the
// returned size.
package seed
