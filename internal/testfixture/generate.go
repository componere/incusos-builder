package testfixture

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

const (
	// Version is the synthetic update version [Generate] writes.
	Version = "202608160000"

	// Format is the apiimages.Index / Update format string on the live server.
	Format = "1.0"

	// Origin is the Update.Origin value observed on the live server.
	Origin = "linuxcontainers.org"

	// ChannelStable is the sole channel membership of the fixture update.
	ChannelStable = "stable"

	// ArchX8664 is the architecture prefix used in fixture filenames.
	ArchX8664 = "x86_64"

	// SeedStart is the seed-data partition start in bytes. Production
	// [build.Build] probes against this constant (internal/build
	// productionSeedStart).
	SeedStart int64 = 2148532224

	// SeedLength is the seed-data partition length (100 MiB).
	SeedLength int64 = 100 << 20

	dirPerm  = 0o755
	filePerm = 0o644
)

// Mirror describes the local-dir update-server tree [Generate] wrote.
type Mirror struct {
	// Dir is the caller-supplied mirror root.
	Dir string
	// Version is the single update version (see [Version]).
	Version string
	// Image is the image-raw UpdateFile admitted by the local adapter.
	Image apiimages.UpdateFile
	// Apps are the application UpdateFiles (arch-prefixed <name>.raw.gz).
	Apps []apiimages.UpdateFile
	// GzipImageSize is the compressed OS image size in bytes.
	GzipImageSize int64
	// RawImageSize is the decompressed OS image size in bytes (~2.10 GiB).
	RawImageSize int64
	// SeedStart is the seed-data partition start in bytes.
	SeedStart int64
	// SeedLength is the seed-data partition length in bytes.
	SeedLength int64
}

// Generate writes a local-dir update-server mirror into dir.
//
// dir is created if needed. The tree is:
//
//	dir/index.json
//	dir/<version>/x86_64/IncusOS_<version>.img.gz
//	dir/<version>/x86_64/debug.raw.gz
//	dir/<version>/x86_64/incus.raw.gz
//	dir/<version>/update.json
//	dir/<version>/update.sjson
//
// Filenames are allowlisted ([A-Za-z0-9._-]+ per slash-separated segment)
// so [update.LocalSource] will open them. Index, update.json, and the
// sjson payload share one Files list (three-way Filename+Sha256+Size
// binding).
func Generate(dir string) (Mirror, error) {
	if dir == "" {
		return Mirror{}, errors.New("testfixture: directory is required")
	}

	err := os.MkdirAll(dir, dirPerm)
	if err != nil {
		return Mirror{}, fmt.Errorf("create fixture directory: %w", err)
	}

	image, rawSize, err := writeOSImage(dir)
	if err != nil {
		return Mirror{}, err
	}

	apps, err := writeApps(dir)
	if err != nil {
		return Mirror{}, err
	}

	files := make([]apiimages.UpdateFile, 0, 1+len(apps))
	files = append(files, image)
	files = append(files, apps...)

	err = writeMetadata(dir, files)
	if err != nil {
		return Mirror{}, err
	}

	return Mirror{
		Dir:           dir,
		Version:       Version,
		Image:         image,
		Apps:          apps,
		GzipImageSize: image.Size,
		RawImageSize:  rawSize,
		SeedStart:     SeedStart,
		SeedLength:    SeedLength,
	}, nil
}

// versionDir is <dir>/<version>.
func versionDir(dir string) string {
	return filepath.Join(dir, Version)
}
