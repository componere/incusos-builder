package cli

import (
	"runtime"
	"slices"
	"time"

	"github.com/spf13/cobra"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/ux"
)

const (
	// flagChannel is the versions --channel flag name.
	flagChannel = "channel"
	// flagArchitecture is the versions --architecture flag name.
	flagArchitecture = "architecture"
)

// versionsResult is the --json success body for versions, written under
// the "result" key.
type versionsResult struct {
	// Versions is the filtered release list. Empty when nothing matches.
	Versions []versionEntry `json:"versions"`
}

// versionEntry is one release in the versions JSON envelope.
type versionEntry struct {
	// Version is the update-server release identifier.
	Version string `json:"version"`
	// Channels is the release's channel membership.
	Channels []string `json:"channels"`
	// PublishedAt is the upstream publication timestamp.
	PublishedAt time.Time `json:"published_at"`
	// Architectures lists architectures that have an image for this release.
	Architectures []string `json:"architectures"`
	// images are the iso/raw files used to render the human table.
	images []versionImage
}

// versionImage is one install-image file on a release.
type versionImage struct {
	// Architecture is x86_64 or aarch64.
	Architecture string
	// Type is iso or raw.
	Type string
}

// versionsCommandName is the cobra Use string for versions.
const versionsCommandName = "versions"

// newVersionsCommand returns the versions subcommand.
func newVersionsCommand(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   versionsCommandName,
		Short: "List available IncusOS releases from the update server",
		Args:  noArgs(opts),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runVersions(cmd, opts)
		},
	}
	cmd.Flags().String(flagChannel, string(build.DefaultChannel), "release channel to list")
	cmd.Flags().String(flagArchitecture, hostArchitecture(), "architecture to list")
	return cmd
}

// runVersions lists releases from the configured update source, filtered
// to the requested channel and to updates that publish an image for the
// requested architecture. An unknown channel is an empty list, not an error.
func runVersions(cmd *cobra.Command, opts Options) error {
	pol, err := resolvePolicy(cmd, opts, opts.Viper)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	channel, err := cmd.Flags().GetString(flagChannel)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	if channel == "" {
		channel = string(build.DefaultChannel)
	}
	arch, err := cmd.Flags().GetString(flagArchitecture)
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	color, progress := reporterModes(pol, false)
	src, err := selectImageSource(pol.Server, pol.CacheDir, ux.New(color, progress, opts.Err))
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	index, err := src.Index(cmd.Context())
	if err != nil {
		return finishCommand(opts, pol, err)
	}
	entries := filterVersions(index, channel, arch)
	if pol.JSON {
		return writeJSON(opts.Out, versionsResult{Versions: entries})
	}
	if pol.Quiet {
		return nil
	}
	ux.VersionsTable(pol.Color, opts.Out, tableRows(entries, channel))
	return nil
}

// filterVersions keeps updates in channel that publish at least one iso/raw
// image for arch. An empty arch keeps every architecture. Unknown channels
// yield an empty slice, not an error.
func filterVersions(index apiimages.Index, channel, arch string) []versionEntry {
	entries := make([]versionEntry, 0)
	for _, rel := range index.Updates {
		if !slices.Contains(rel.Channels, channel) {
			continue
		}
		images := matchingImages(rel.Files, arch)
		if len(images) == 0 {
			continue
		}
		channels := append([]string(nil), rel.Channels...)
		if channels == nil {
			channels = []string{}
		}
		entries = append(entries, versionEntry{
			Version:       rel.Version,
			Channels:      channels,
			PublishedAt:   rel.PublishedAt,
			Architectures: uniqueArchitectures(images),
			images:        images,
		})
	}
	return entries
}

// matchingImages returns iso/raw files, optionally restricted to
// architecture want.
func matchingImages(files []apiimages.UpdateFile, want string) []versionImage {
	images := make([]versionImage, 0)
	seen := make(map[versionImage]struct{})
	for _, file := range files {
		typ, ok := imageTypeName(file.Type)
		if !ok {
			continue
		}
		arch := string(file.Architecture)
		if want != "" && arch != want {
			continue
		}
		img := versionImage{Architecture: arch, Type: typ}
		if _, dup := seen[img]; dup {
			continue
		}
		seen[img] = struct{}{}
		images = append(images, img)
	}
	return images
}

// uniqueArchitectures preserves first-seen architecture order from images.
func uniqueArchitectures(images []versionImage) []string {
	seen := make(map[string]struct{})
	archs := make([]string, 0)
	for _, img := range images {
		if _, ok := seen[img.Architecture]; ok {
			continue
		}
		seen[img.Architecture] = struct{}{}
		archs = append(archs, img.Architecture)
	}
	return archs
}

// tableRows flattens JSON entries into one VersionsTable row per image type.
func tableRows(entries []versionEntry, channel string) []ux.VersionRow {
	rows := make([]ux.VersionRow, 0)
	for _, entry := range entries {
		for _, img := range entry.images {
			rows = append(rows, ux.VersionRow{
				Version:      entry.Version,
				Channel:      channel,
				Architecture: img.Architecture,
				Type:         img.Type,
			})
		}
	}
	return rows
}

// imageTypeName maps an update-server file type to the CLI image type.
func imageTypeName(t apiimages.UpdateFileType) (string, bool) {
	switch t {
	case apiimages.UpdateFileTypeImageISO:
		return string(build.ImageTypeISO), true
	case apiimages.UpdateFileTypeImageRaw:
		return string(build.ImageTypeRaw), true
	case apiimages.UpdateFileTypeUndefined,
		apiimages.UpdateFileTypeImageManifest,
		apiimages.UpdateFileTypeChangelog,
		apiimages.UpdateFileTypeUpdateEFI,
		apiimages.UpdateFileTypeUpdateUsr,
		apiimages.UpdateFileTypeUpdateUsrVerity,
		apiimages.UpdateFileTypeUpdateUsrVeritySignature,
		apiimages.UpdateFileTypeUpdateSecureboot,
		apiimages.UpdateFileTypeApplication:
		return "", false
	default:
		return "", false
	}
}

// hostArchitecture maps [runtime.GOARCH] onto the update-server names.
// amd64 becomes x86_64, arm64 becomes aarch64; any other value becomes x86_64.
func hostArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return string(build.ArchX8664)
	case "arm64":
		return string(build.ArchAarch64)
	default:
		return string(build.ArchX8664)
	}
}
