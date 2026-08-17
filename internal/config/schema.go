package config

import (
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"

	"github.com/componere/incusos-builder/internal/build"
)

// currentSchemaVersion is the only document version this CLI accepts.
const currentSchemaVersion = 1

// defaultSeedVersion is applied to each present seed that omits version.
const defaultSeedVersion = "1"

// pinnedIncusOS is the incus-osd module version whose types define the YAML
// schema.
const pinnedIncusOS = "v0.0.0-20260815030500-0f5b8057f2fc"

// unknownFieldHint is appended to strict-decode errors for fields the pin does
// not know.
const unknownFieldHint = "unknown to incus-os " + pinnedIncusOS + "; a newer incusos-builder may accept this"

// diskEncryptionDocs is the upstream page that documents encryption recovery
// keys.
const diskEncryptionDocs = "https://linuxcontainers.org/incus-os/docs/main/reference/system/security/"

// stdinPath selects [os.Stdin] in [Load].
const stdinPath = "-"

// document is the decoded version-1 YAML config.
type document struct {
	// Version is the config schema version; required and must equal 1.
	Version *int `yaml:"version"`
	// Image selects the artifact to build.
	Image image `yaml:"image"`
	// Seeds holds optional seed sections. Nil pointers are omitted.
	Seeds seeds `yaml:"seeds"`
}

// image selects the artifact type, architecture, channel, release, and
// offline flag.
type image struct {
	// Type is iso or raw.
	Type build.ImageType `yaml:"type"`
	// Architecture is x86_64 or aarch64.
	Architecture build.Architecture `yaml:"architecture"`
	// Channel filters update-server releases; omitted defaults to stable.
	Channel build.Channel `yaml:"channel"`
	// Release pins an exact update version; empty selects the highest in
	// Channel.
	Release build.Release `yaml:"release"`
	// Offline builds RESCUE_DATA resources media in addition to the image.
	Offline bool `yaml:"offline"`
}

// seeds embeds the imported upstream seed types. YAML names match
// customizer.ImagesPostSeeds, including kebab-case migration-manager
// and operations-center. Kernel and Security are CLI-only extensions.
type seeds struct {
	// Applications is the optional applications seed. Offline images require a
	// non-empty list.
	Applications *apiseed.Applications `yaml:"applications"`
	// Incus is the optional Incus init preseed.
	Incus *apiseed.Incus `yaml:"incus"`
	// Install is the optional installer seed, including target selection.
	Install *apiseed.Install `yaml:"install"`
	// MigrationManager is the optional migration-manager seed.
	MigrationManager *apiseed.MigrationManager `yaml:"migration-manager"`
	// Network is the optional network seed.
	Network *apiseed.Network `yaml:"network"`
	// OperationsCenter is the optional operations-center seed.
	OperationsCenter *apiseed.OperationsCenter `yaml:"operations-center"`
	// Provider is the optional provider-registration seed.
	Provider *apiseed.Provider `yaml:"provider"`
	// Services is the optional auxiliary-service seed.
	Services *apiseed.Services `yaml:"services"`
	// Update is the optional update-daemon seed. Offline images force
	// check_frequency to never.
	Update *apiseed.Update `yaml:"update"`
	// Kernel is the optional kernel seed, a CLI-only extension.
	Kernel *apiseed.Kernel `yaml:"kernel"`
	// Security is the optional security seed, a CLI-only extension.
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
