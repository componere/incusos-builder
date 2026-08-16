package build

import (
	"fmt"
	"path"
	"slices"
	"strings"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

const appSuffix = ".raw.gz"

// Resolve selects a release from index the way upstream filterAssets does
// (image-customizer main.go:823): channel membership, an exact Release pin
// or the highest version by string compare of upstream version names, then
// exactly one image-iso/image-raw for the spec's type and architecture.
// Application assets are matched as <name>.raw.gz using path.Base so
// per-arch prefixes (aarch64/incus.raw.gz) still hit. A missing application
// wraps [ErrVersionNotFound] and lists what the update does carry.
func Resolve(spec Spec, index apiimages.Index) (Plan, error) {
	channel := string(spec.Channel)
	if channel == "" {
		channel = string(DefaultChannel)
	}

	update, err := selectUpdate(index, channel, string(spec.Release))
	if err != nil {
		return Plan{}, err
	}

	fileType, err := imageFileType(spec.Type)
	if err != nil {
		return Plan{}, err
	}

	arch := apiimages.UpdateFileArchitecture(spec.Architecture)
	images := matchingFiles(update.Files, arch, fileType)
	if len(images) != 1 {
		return Plan{}, fmt.Errorf(
			"%w: expected exactly one %s image for %s in %s, found %d",
			ErrVersionNotFound,
			fileType,
			spec.Architecture,
			update.Version,
			len(images),
		)
	}

	apps, err := matchApplications(spec, update)
	if err != nil {
		return Plan{}, err
	}

	return Plan{
		Version: update.Version,
		Image:   images[0],
		Apps:    apps,
	}, nil
}

// selectUpdate returns the pinned update or the highest-version update in
// channel. Version comparison is a plain string compare, matching
// filterAssets (main.go:842: `update.Version > highestVersion`).
func selectUpdate(index apiimages.Index, channel, pin string) (apiimages.UpdateFull, error) {
	highestVersion := ""
	highestIdx := -1

	for i, update := range index.Updates {
		if !slices.Contains(update.Channels, channel) {
			continue
		}

		if pin != "" && update.Version == pin {
			highestIdx = i

			break
		}

		if update.Version > highestVersion {
			highestVersion = update.Version
			highestIdx = i
		}
	}

	available := channelVersions(index, channel)

	if highestIdx < 0 || (pin != "" && index.Updates[highestIdx].Version != pin) {
		if pin != "" {
			return apiimages.UpdateFull{}, fmt.Errorf(
				"%w: release %q not in channel %q; available: %s",
				ErrVersionNotFound,
				pin,
				channel,
				joinOrNone(available),
			)
		}

		return apiimages.UpdateFull{}, fmt.Errorf(
			"%w: no updates in channel %q; available: %s",
			ErrVersionNotFound,
			channel,
			joinOrNone(available),
		)
	}

	return index.Updates[highestIdx], nil
}

// imageFileType maps a spec image type to the update-server file type.
func imageFileType(t ImageType) (apiimages.UpdateFileType, error) {
	switch t {
	case ImageTypeISO:
		return apiimages.UpdateFileTypeImageISO, nil
	case ImageTypeRaw:
		return apiimages.UpdateFileTypeImageRaw, nil
	default:
		return apiimages.UpdateFileTypeUndefined, fmt.Errorf("%w: unknown image type %q", ErrVersionNotFound, t)
	}
}

// matchingFiles returns files of fileType for arch, preserving index order.
func matchingFiles(
	files []apiimages.UpdateFile,
	arch apiimages.UpdateFileArchitecture,
	fileType apiimages.UpdateFileType,
) []apiimages.UpdateFile {
	matched := make([]apiimages.UpdateFile, 0)

	for _, file := range files {
		if file.Architecture != arch {
			continue
		}

		if file.Type != fileType {
			continue
		}

		matched = append(matched, file)
	}

	return matched
}

// matchApplications resolves each requested application to the update file
// whose basename is <name>.raw.gz (sendRescueImage main.go:619–628).
func matchApplications(spec Spec, update apiimages.UpdateFull) ([]apiimages.UpdateFile, error) {
	if spec.Seeds.Applications == nil {
		return nil, nil
	}

	arch := apiimages.UpdateFileArchitecture(spec.Architecture)
	appFiles := matchingFiles(update.Files, arch, apiimages.UpdateFileTypeApplication)
	carried := make([]string, 0, len(appFiles))

	for _, file := range appFiles {
		carried = append(carried, file.Filename)
	}

	matched := make([]apiimages.UpdateFile, 0, len(spec.Seeds.Applications.Applications))

	for _, app := range spec.Seeds.Applications.Applications {
		want := app.Name + appSuffix
		file, ok := findAppFile(appFiles, want)
		if !ok {
			return nil, fmt.Errorf(
				"%w: application %q not in update %s; update does carry: %s",
				ErrVersionNotFound,
				app.Name,
				update.Version,
				joinOrNone(carried),
			)
		}

		matched = append(matched, file)
	}

	return matched, nil
}

// findAppFile returns the first file whose basename equals want.
func findAppFile(files []apiimages.UpdateFile, want string) (apiimages.UpdateFile, bool) {
	for _, file := range files {
		if path.Base(file.Filename) == want {
			return file, true
		}
	}

	return apiimages.UpdateFile{}, false
}

// channelVersions lists unique versions whose Channels contain channel.
func channelVersions(index apiimages.Index, channel string) []string {
	seen := make(map[string]struct{})
	versions := make([]string, 0)

	for _, update := range index.Updates {
		if !slices.Contains(update.Channels, channel) {
			continue
		}

		if _, ok := seen[update.Version]; ok {
			continue
		}

		seen[update.Version] = struct{}{}
		versions = append(versions, update.Version)
	}

	slices.Sort(versions)

	return versions
}

// joinOrNone joins items with ", " or returns "none" when empty.
func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}

	return strings.Join(items, ", ")
}
