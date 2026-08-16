package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/cli"
	"github.com/componere/incusos-builder/internal/config"
	"github.com/componere/incusos-builder/internal/update"
	"github.com/componere/incusos-builder/internal/ux"
)

// TestLiveVersionsParsesDefaultServer lists the live HTTPS index through the
// versions CLI and requires a machine-readable release list.
func TestLiveVersionsParsesDefaultServer(t *testing.T) {
	requireLiveE2E(t)

	stdout := runLiveCLI(t, nil, "versions", "--json", "--color", "never", "--progress", "never")
	env := decodeVersionsEnvelope(t, stdout)
	require.NotEmpty(t, env.Result.Versions, "live versions should list at least one release")
	for _, entry := range env.Result.Versions {
		assert.NotEmpty(t, entry.Version, "version name")
		assert.NotEmpty(t, entry.Architectures, "architectures for %s", entry.Version)
		assert.False(t, entry.PublishedAt.IsZero(), "published_at for %s", entry.Version)
	}
}

// TestLiveSmallestRawSeedRoundTrip builds the smallest live raw image with
// every seed section and checks the spliced seed-data partition.
func TestLiveSmallestRawSeedRoundTrip(t *testing.T) {
	requireLiveE2E(t)

	cacheDir := t.TempDir()
	image, index := smallestLiveRaw(t, cacheDir)
	cfgPath := writeLiveConfig(t, liveOnlineRawConfig(image))
	outPath := filepath.Join(t.TempDir(), "seeded.img")

	stdout := runLiveCLI(t, nil,
		"build", "--json",
		"-f", cfgPath,
		"-o", outPath,
		"--cache-dir", cacheDir,
		"--color", "never",
		"--progress", "never",
	)
	result := decodeBuildEnvelope(t, stdout)
	assert.Equal(t, outPath, result.Output)
	assert.Equal(t, "raw", result.Type)
	assert.Equal(t, image.Architecture, result.Architecture)
	assert.Equal(t, image.Version, result.Version)
	assert.Equal(t, "stable", result.Channel)
	assert.Empty(t, result.ResourcesOutput)
	assert.NotEmpty(t, result.SHA256)
	assert.Equal(t, result.SHA256, fileSHA256Hex(t, outPath), "published digest must match streamed file hash")
	assert.Equal(t, image.Version, selectedUpdate(t, index, image.Version).Version)

	want, err := config.Parse([]byte(liveOnlineRawConfig(image)))
	require.NoError(t, err, "fixture config must parse")
	assertSeedRoundTrip(t, outPath, want.Seeds, result.SeedBytes)
}

// TestLiveOfflineRescueReadBack builds offline iso and raw rescue media and
// checks the go-diskfs read-back of every staged update asset.
func TestLiveOfflineRescueReadBack(t *testing.T) {
	requireLiveE2E(t)

	cacheDir := t.TempDir()
	index := liveIndex(t, cacheDir)

	tests := []struct {
		name string
		typ  string
		ext  string
	}{
		{name: "iso", typ: "iso", ext: "iso"},
		{name: "raw", typ: "raw", ext: "img"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work := t.TempDir()
			image := smallestLiveImage(t, index, imageFileType(t, tt.typ))
			app := smallestLiveApplication(t, index, image)
			meta := fetchReleaseMetadata(t, cacheDir, image.Version, []apiimages.UpdateFile{app.File})
			cfgPath := writeLiveConfig(t, liveOfflineConfig(tt.typ, image, app.Name))
			outPath := filepath.Join(work, "install."+tt.ext)
			resourcesPath := filepath.Join(work, "install.resources."+tt.ext)

			stdout := runLiveCLI(t, nil,
				"build", "--json",
				"-f", cfgPath,
				"-o", outPath,
				"--resources-output", resourcesPath,
				"--cache-dir", cacheDir,
				"--color", "never",
				"--progress", "never",
			)
			result := decodeBuildEnvelope(t, stdout)
			assert.Equal(t, outPath, result.Output)
			assert.Equal(t, resourcesPath, result.ResourcesOutput)
			assert.Equal(t, tt.typ, result.Type)
			assert.Equal(t, image.Architecture, result.Architecture)
			assert.Equal(t, image.Version, result.Version)
			assert.NotEmpty(t, result.SHA256)
			assert.NotEmpty(t, result.ResourcesSHA256)
			assert.Equal(t, result.SHA256, fileSHA256Hex(t, outPath))
			assert.Equal(t, result.ResourcesSHA256, fileSHA256Hex(t, resourcesPath))

			got := readRescueTree(t, resourcesPath, tt.typ == "iso")
			wantRel := rescueUpdatePath(app.File.Filename)
			require.Contains(t, got, "update/update.json")
			require.Contains(t, got, "update/update.sjson")
			require.Contains(t, got, wantRel)
			assert.Equal(t, meta.UpdateJSON, got["update/update.json"], "update.json bytes")
			assert.Equal(t, meta.UpdateSJSON, got["update/update.sjson"], "update.sjson bytes")
			assert.Equal(t, fileSHA256Hex(t, cachedAssetPath(cacheDir, app.File.Sha256)),
				sha256OfRescueFile(t, got, wantRel), "application asset digest")
			assert.Len(t, got, 3, "rescue tree should contain only staged files")
		})
	}
}

// liveImage is one raw install image chosen from the live index.
type liveImage struct {
	// Version is the update version that published the image.
	Version string
	// Architecture is the update-server architecture name.
	Architecture string
	// File is the image-raw asset.
	File apiimages.UpdateFile
}

// liveApplication is one application asset from the same update as liveImage.
type liveApplication struct {
	// Name is the application stem without the .raw.gz suffix.
	Name string
	// File is the application asset.
	File apiimages.UpdateFile
}

// versionsEnvelope is the --json success document from versions.
type versionsEnvelope struct {
	// Result is the success body.
	Result versionsJSON `json:"result"`
}

// versionsJSON is the versions success body.
type versionsJSON struct {
	// Versions is the filtered release list.
	Versions []versionJSON `json:"versions"`
}

// versionJSON is one release in the versions JSON envelope.
type versionJSON struct {
	// Version is the update version name.
	Version string `json:"version"`
	// Channels is the release's channel membership.
	Channels []string `json:"channels"`
	// PublishedAt is the upstream publication timestamp.
	PublishedAt time.Time `json:"published_at"`
	// Architectures lists architectures that have an image for this release.
	Architectures []string `json:"architectures"`
}

// buildEnvelopeJSON is the --json success document from build.
type buildEnvelopeJSON struct {
	// Result is the success body.
	Result buildResultJSON `json:"result"`
}

// buildResultJSON names the published artifacts and the resolved image.
type buildResultJSON struct {
	// Output is the -o path.
	Output string `json:"output"`
	// ResourcesOutput is the --resources-output path.
	ResourcesOutput string `json:"resources_output"`
	// Type is the image type.
	Type string `json:"type"`
	// Architecture is the CPU architecture.
	Architecture string `json:"architecture"`
	// Version is the resolved update version.
	Version string `json:"version"`
	// Channel is the channel the version was selected from.
	Channel string `json:"channel"`
	// SeedBytes is the spliced seed-tar size.
	SeedBytes int64 `json:"seed_bytes"`
	// SHA256 is the stored image digest.
	SHA256 string `json:"sha256"`
	// ResourcesSHA256 is the resources-media digest.
	ResourcesSHA256 string `json:"resources_sha256"`
}

// runLiveCLI executes the production command tree against the default live
// server and returns stdout. stderr is discarded unless the command fails.
func runLiveCLI(t *testing.T, stdin []byte, args ...string) string {
	t.Helper()

	var stdout, stderr bytes.Buffer
	opts := cli.Options{
		In:        bytes.NewReader(stdin),
		Out:       &stdout,
		Err:       &stderr,
		Viper:     viper.New(),
		StdinTTY:  func() bool { return false },
		StdoutTTY: func() bool { return false },
		StderrTTY: func() bool { return false },
	}
	root := cli.NewRootCommand(opts)
	root.SetArgs(args)
	err := root.ExecuteContext(t.Context())
	require.NoError(t, err, "cli %v failed: %s", args, stderr.String())
	return stdout.String()
}

// decodeVersionsEnvelope parses the versions --json envelope.
func decodeVersionsEnvelope(t *testing.T, raw string) versionsEnvelope {
	t.Helper()

	var env versionsEnvelope
	require.NoError(t, json.Unmarshal([]byte(raw), &env), "versions JSON")
	require.NotNil(t, env.Result.Versions, "versions list")
	return env
}

// decodeBuildEnvelope parses the build --json envelope.
func decodeBuildEnvelope(t *testing.T, raw string) buildResultJSON {
	t.Helper()

	var env buildEnvelopeJSON
	require.NoError(t, json.Unmarshal([]byte(raw), &env), "build JSON")
	require.NotEmpty(t, env.Result.Output, "result.output")
	return env.Result
}

// liveIndex fetches the default HTTPS index through the production adapter.
func liveIndex(t *testing.T, cacheDir string) apiimages.Index {
	t.Helper()

	src, err := update.NewHTTPSSource(
		liveDefaultServer,
		cacheDir,
		ux.New(ux.ColorModeNever, ux.ProgressModeNever, os.Stderr),
		nil,
	)
	require.NoError(t, err, "open live HTTPS source")
	index, err := src.Index(t.Context())
	require.NoError(t, err, "fetch live index.json")
	require.NotEmpty(t, index.Updates, "live index should list updates")
	return index
}

func smallestLiveRaw(t *testing.T, cacheDir string) (liveImage, apiimages.Index) {
	t.Helper()

	index := liveIndex(t, cacheDir)
	return smallestLiveImage(t, index, apiimages.UpdateFileTypeImageRaw), index
}

// smallestLiveImage selects the smallest file of typ on the live index.
func smallestLiveImage(t *testing.T, index apiimages.Index, typ apiimages.UpdateFileType) liveImage {
	t.Helper()

	var chosen liveImage
	var size int64 = -1
	for _, rel := range index.Updates {
		if !slices.Contains(rel.Channels, "stable") {
			continue
		}
		for _, file := range rel.Files {
			if file.Type != typ {
				continue
			}
			if size >= 0 && file.Size >= size {
				continue
			}
			size = file.Size
			chosen = liveImage{
				Version:      rel.Version,
				Architecture: string(file.Architecture),
				File:         file,
			}
		}
	}
	require.NotEmpty(t, chosen.Version, "live index should publish at least one %s image", typ)
	require.Positive(t, chosen.File.Size, "%s image size", typ)
	return chosen
}

// imageFileType maps a CLI image type onto the update-server file type.
func imageFileType(t *testing.T, typ string) apiimages.UpdateFileType {
	t.Helper()

	switch typ {
	case "iso":
		return apiimages.UpdateFileTypeImageISO
	case "raw":
		return apiimages.UpdateFileTypeImageRaw
	default:
		t.Fatalf("unknown image type %q", typ)
		return apiimages.UpdateFileTypeUndefined
	}
}

// smallestLiveApplication picks the smallest application on the selected update.
func smallestLiveApplication(t *testing.T, index apiimages.Index, image liveImage) liveApplication {
	t.Helper()

	rel := selectedUpdate(t, index, image.Version)
	var chosen liveApplication
	var size int64 = -1
	for _, file := range rel.Files {
		if file.Type != apiimages.UpdateFileTypeApplication {
			continue
		}
		if string(file.Architecture) != image.Architecture {
			continue
		}
		if size >= 0 && file.Size >= size {
			continue
		}
		name := applicationName(file.Filename)
		if name == "" {
			continue
		}
		size = file.Size
		chosen = liveApplication{Name: name, File: file}
	}
	require.NotEmpty(
		t,
		chosen.Name,
		"update %s should publish an application for %s",
		image.Version,
		image.Architecture,
	)
	return chosen
}

// selectedUpdate returns the named update from index.
func selectedUpdate(t *testing.T, index apiimages.Index, version string) apiimages.UpdateFull {
	t.Helper()

	for _, rel := range index.Updates {
		if rel.Version == version {
			return rel
		}
	}
	t.Fatalf("update %s missing from live index", version)
	return apiimages.UpdateFull{}
}

// fetchReleaseMetadata downloads the verbatim rescue metadata for selected.
func fetchReleaseMetadata(
	t *testing.T,
	cacheDir, version string,
	selected []apiimages.UpdateFile,
) build.ReleaseMetadata {
	t.Helper()

	src, err := update.NewHTTPSSource(
		liveDefaultServer,
		cacheDir,
		ux.New(ux.ColorModeNever, ux.ProgressModeNever, os.Stderr),
		nil,
	)
	require.NoError(t, err, "open live HTTPS source")
	meta, err := src.ReleaseMetadata(context.Background(), version, selected)
	require.NoError(t, err, "fetch release metadata")
	require.NotEmpty(t, meta.UpdateJSON, "update.json")
	require.NotEmpty(t, meta.UpdateSJSON, "update.sjson")
	return meta
}

// writeLiveConfig writes body into a temp YAML file.
func writeLiveConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}
