package build

import (
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// ImageType selects the artifact layout of the built image.
type ImageType string

// Image types accepted by the update server's asset naming (image-iso /
// image-raw) and by upstream's customizer.
const (
	// ImageTypeISO is the iso9660 installer artifact.
	ImageTypeISO ImageType = "iso"
	// ImageTypeRaw is the raw disk artifact.
	ImageTypeRaw ImageType = "raw"
)

// Architecture is an update-server CPU architecture name.
type Architecture string

// Architectures accepted by the update server.
const (
	// ArchX8664 is the x86_64 architecture.
	ArchX8664 Architecture = "x86_64"
	// ArchAarch64 is the aarch64 architecture.
	ArchAarch64 Architecture = "aarch64"
)

// Channel names an update-server release channel (free text upstream;
// default "stable").
type Channel string

// DefaultChannel is applied when the config omits image.channel.
const DefaultChannel Channel = "stable"

// Release is an exact update version pin (upstream ImagesPost.Version).
// Empty means "highest version in the channel".
type Release string

// Spec is the fully validated build specification produced by
// internal/config and consumed by Build. It carries no I/O handles.
type Spec struct {
	// Type selects iso or raw output.
	Type ImageType
	// Architecture selects the update-server architecture.
	Architecture Architecture
	// Channel filters candidate updates; default "stable".
	Channel Channel
	// Release pins an exact version; empty selects the highest in Channel.
	Release Release
	// Offline additionally builds RESCUE_DATA resources media.
	Offline bool
	// Seeds holds every seed section to splice into the image.
	Seeds Seeds
}

// Seeds aggregates all eleven seed sections incus-osd reads from the
// seed-data partition. The nine web-API sections mirror upstream
// customizer.ImagesPostSeeds field-for-field; Kernel and Security are
// CLI-exclusive extensions (the web service cannot emit them). Nil sections
// are omitted from the seed tar.
type Seeds struct {
	// Applications seeds preinstalled applications.
	Applications *apiseed.Applications
	// Incus seeds the full Incus init preseed.
	Incus *apiseed.Incus
	// Install seeds the installer (target selection, security flags).
	Install *apiseed.Install
	// MigrationManager seeds the migration-manager service.
	MigrationManager *apiseed.MigrationManager
	// Network seeds the network configuration.
	Network *apiseed.Network
	// OperationsCenter seeds the operations-center service.
	OperationsCenter *apiseed.OperationsCenter
	// Provider seeds the provider registration.
	Provider *apiseed.Provider
	// Services seeds auxiliary service toggles.
	Services *apiseed.Services
	// Update seeds the update daemon configuration.
	Update *apiseed.Update
	// Kernel seeds kernel configuration (CLI extension).
	Kernel *apiseed.Kernel
	// Security seeds security configuration (CLI extension; validation
	// rejects non-empty encryption_recovery_keys because incus-osd fatally
	// rejects them at boot).
	Security *apiseed.Security
}
