package build

import (
	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

// Plan is the resolved release: one OS image file plus the application
// files requested by the spec. Resolve produces it from a Spec and an
// update-server index; Build consumes it.
type Plan struct {
	// Version is the selected update version (the pin, or the highest
	// version in the channel).
	Version string
	// Image is the unique image-iso or image-raw asset for the spec's
	// type and architecture.
	Image apiimages.UpdateFile
	// Apps are the application assets matched as <name>.raw.gz against
	// the selected update, in spec order. Filenames keep the per-arch
	// prefix published by the server (for example aarch64/incus.raw.gz).
	// Empty unless spec.Offline; online builds skip application matching.
	Apps []apiimages.UpdateFile
}

// Result is what Build reports after a successful run. Digests and final
// paths are a publication concern of internal/cli, not the domain.
type Result struct {
	// Version is the resolved update version written into the image.
	Version string
	// Channel is the channel the version was selected from.
	Channel Channel
	// Type is the image type that was built.
	Type ImageType
	// Architecture is the architecture that was built.
	Architecture Architecture
	// BytesWritten is the number of decompressed bytes written to out
	// (the spliced OS image).
	BytesWritten int64
	// SeedBytes is the size of the spliced seed tar.
	SeedBytes int64
	// Offline is true when rescue media was also produced.
	Offline bool
	// ResourcesTmp is the caller-owned temp path rescue media was written
	// to. Empty when Offline is false.
	ResourcesTmp string
}
