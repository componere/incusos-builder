package config

import (
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"

	"github.com/componere/incusos-builder/internal/build"
)

// currentSchemaVersion is the only document version this CLI accepts.
const currentSchemaVersion = 1

// defaultSeedVersion is applied to each present seed that omits version,
// matching the web customizer (local.js).
const defaultSeedVersion = "1"

// pinnedIncusOS is the incus-osd module version whose types define the YAML schema.
const pinnedIncusOS = "v0.0.0-20260815030500-0f5b8057f2fc"

// unknownFieldHint is appended to strict-decode errors for fields the pin does not know.
const unknownFieldHint = "unknown to incus-os " + pinnedIncusOS + "; a newer incusos-builder may accept this"

// diskEncryptionDocs is the upstream page that documents encryption recovery keys.
const diskEncryptionDocs = "https://linuxcontainers.org/incus-os/docs/main/reference/system/security/"

// stdinPath selects os.Stdin in [Load].
const stdinPath = "-"

// document is the version: 1 config file decoded from YAML.
type document struct {
	// Version is the config schema version; required and must equal 1.
	Version *int `yaml:"version"`
	// Image selects the artifact to build.
	Image image `yaml:"image"`
	// Seeds holds optional seed sections. Nil pointers are omitted.
	Seeds seeds `yaml:"seeds"`
}

// image is the image: mapping.
type image struct {
	// Type is iso or raw.
	Type build.ImageType `yaml:"type"`
	// Architecture is x86_64 or aarch64.
	Architecture build.Architecture `yaml:"architecture"`
	// Channel filters update-server releases; omitted defaults to stable.
	Channel build.Channel `yaml:"channel"`
	// Release pins an exact update version; empty selects the highest in Channel.
	Release build.Release `yaml:"release"`
	// Offline also builds RESCUE_DATA resources media.
	Offline bool `yaml:"offline"`
}

// seeds embeds the imported upstream seed types. YAML names match
// customizer.ImagesPostSeeds, including kebab-case migration-manager
// and operations-center. Kernel and Security are CLI-only extensions.
type seeds struct {
	// Applications seeds preinstalled applications.
	Applications *apiseed.Applications `yaml:"applications"`
	// Incus seeds the full Incus init preseed.
	Incus *apiseed.Incus `yaml:"incus"`
	// Install seeds the installer.
	Install *apiseed.Install `yaml:"install"`
	// MigrationManager seeds the migration-manager service.
	MigrationManager *apiseed.MigrationManager `yaml:"migration-manager"`
	// Network seeds the network configuration.
	Network *apiseed.Network `yaml:"network"`
	// OperationsCenter seeds the operations-center service.
	OperationsCenter *apiseed.OperationsCenter `yaml:"operations-center"`
	// Provider seeds the provider registration.
	Provider *apiseed.Provider `yaml:"provider"`
	// Services seeds auxiliary service toggles.
	Services *apiseed.Services `yaml:"services"`
	// Update seeds the update daemon configuration.
	Update *apiseed.Update `yaml:"update"`
	// Kernel seeds kernel configuration (CLI extension).
	Kernel *apiseed.Kernel `yaml:"kernel"`
	// Security seeds security configuration (CLI extension).
	Security *apiseed.Security `yaml:"security"`
}

// spec converts the decoded document into a [build.Spec].
func (d document) spec() build.Spec {
	return build.Spec{
		Type:         d.Image.Type,
		Architecture: d.Image.Architecture,
		Channel:      d.Image.Channel,
		Release:      d.Image.Release,
		Offline:      d.Image.Offline,
		Seeds: build.Seeds{
			Applications:     d.Seeds.Applications,
			Incus:            d.Seeds.Incus,
			Install:          d.Seeds.Install,
			MigrationManager: d.Seeds.MigrationManager,
			Network:          d.Seeds.Network,
			OperationsCenter: d.Seeds.OperationsCenter,
			Provider:         d.Seeds.Provider,
			Services:         d.Seeds.Services,
			Update:           d.Seeds.Update,
			Kernel:           d.Seeds.Kernel,
			Security:         d.Seeds.Security,
		},
	}
}
