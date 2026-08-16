package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	oldImageBody      = "OLDIMAGE"
	oldResourcesBody  = "OLDRES"
	newImageBody      = "NEWIMAGE"
	newResourcesBody  = "NEWRES"
	sneakBody         = "sneak"
	imageName         = "out.iso"
	resourcesName     = "out.resources.iso"
	rawImageName      = "disk.img"
	filePerm          = 0o644
	concurrentWorkers = 8
)

// TestResolvePathsDefaultResourcesNames covers default <stem>.resources.<iso|img>
// naming and cleaned-path distinctness, including stdout rejection.
func TestResolvePathsDefaultResourcesNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	iso := filepath.Join(dir, imageName)
	raw := filepath.Join(dir, rawImageName)
	nested := filepath.Join(dir, "foo.bar.iso")

	tests := []struct {
		name      string
		image     string
		resources string
		offline   bool
		typ       build.ImageType
		wantImg   string
		wantRes   string
		wantErr   error
		contain   string
	}{
		{
			name:    "offline iso defaults beside -o",
			image:   iso,
			offline: true,
			typ:     build.ImageTypeISO,
			wantImg: filepath.Clean(iso),
			wantRes: filepath.Join(dir, resourcesName),
		},
		{
			name:    "offline raw defaults to .img",
			image:   raw,
			offline: true,
			typ:     build.ImageTypeRaw,
			wantImg: filepath.Clean(raw),
			wantRes: filepath.Join(dir, "disk.resources.img"),
		},
		{
			name:    "extension follows type not -o suffix",
			image:   iso,
			offline: true,
			typ:     build.ImageTypeRaw,
			wantImg: filepath.Clean(iso),
			wantRes: filepath.Join(dir, "out.resources.img"),
		},
		{
			name:    "multi-dot stem",
			image:   nested,
			offline: true,
			typ:     build.ImageTypeISO,
			wantImg: filepath.Clean(nested),
			wantRes: filepath.Join(dir, "foo.bar.resources.iso"),
		},
		{
			name:      "online ignores resources flag",
			image:     iso,
			resources: filepath.Join(dir, "ignored.iso"),
			offline:   false,
			typ:       build.ImageTypeISO,
			wantImg:   filepath.Clean(iso),
			wantRes:   "",
		},
		{
			name:      "explicit resources kept",
			image:     iso,
			resources: filepath.Join(dir, "custom.iso"),
			offline:   true,
			typ:       build.ImageTypeISO,
			wantImg:   filepath.Clean(iso),
			wantRes:   filepath.Join(dir, "custom.iso"),
		},
		{
			name:      "cleaned ./image matches image",
			image:     "out.iso",
			resources: "./out.iso",
			offline:   true,
			typ:       build.ImageTypeISO,
			wantErr:   ErrUsage,
			contain:   "distinct",
		},
		{
			name:      "same path after clean",
			image:     iso,
			resources: iso,
			offline:   true,
			typ:       build.ImageTypeISO,
			wantErr:   ErrUsage,
			contain:   "distinct",
		},
		{
			name:    "image stdout sentinel",
			image:   stdoutSentinel,
			offline: false,
			typ:     build.ImageTypeISO,
			wantErr: ErrUsage,
			contain: stdoutSentinel,
		},
		{
			name:      "resources stdout sentinel",
			image:     iso,
			resources: stdoutSentinel,
			offline:   true,
			typ:       build.ImageTypeISO,
			wantErr:   ErrUsage,
			contain:   stdoutSentinel,
		},
		{
			name:    "empty image",
			image:   "  ",
			offline: false,
			wantErr: ErrUsage,
			contain: "required",
		},
		{
			name:    "unknown type when defaulting",
			image:   iso,
			offline: true,
			typ:     build.ImageType("qcow2"),
			wantErr: ErrUsage,
			contain: "unknown image type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolvePaths(tt.image, tt.resources, tt.offline, tt.typ)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.True(t, IsUsage(err))
				if tt.contain != "" {
					require.Contains(t, err.Error(), tt.contain)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantImg, got.Image)
			require.Equal(t, tt.wantRes, got.Resources)
		})
	}
}

// TestRefuseOverwriteWithoutForce is the pre-work existence check (UX, exit 2).
func TestRefuseOverwriteWithoutForce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	image := filepath.Join(dir, imageName)
	writeFile(t, image, oldImageBody)

	_, err := Begin(Request{
		Image:   image,
		Offline: false,
		Type:    build.ImageTypeISO,
	})
	require.ErrorIs(t, err, ErrUsage)
	require.Contains(t, err.Error(), "refusing to overwrite")
	require.Equal(t, oldImageBody, readFile(t, image))
}

// TestConfirmFalseIsUsage refuses when the injected confirm callback declines.
func TestConfirmFalseIsUsage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	image := filepath.Join(dir, imageName)
	writeFile(t, image, oldImageBody)

	_, err := Begin(Request{
		Image:   image,
		Type:    build.ImageTypeISO,
		Confirm: func() (bool, error) { return false, nil },
	})
	require.ErrorIs(t, err, ErrUsage)
	require.Equal(t, oldImageBody, readFile(t, image))
}

// TestClaimRaceRefusesOutputAppeared covers a file appearing after the
// pre-check: no-clobber claim fails with the exit-6 wording and does not
// overwrite the interloper.
func TestClaimRaceRefusesOutputAppeared(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	session := beginOffline(t, dir, false)
	image := session.Paths().Image
	resources := session.Paths().Resources
	writePayloads(t, session)

	writeFile(t, image, sneakBody)
	writeFile(t, resources, sneakBody)

	_, err := session.Publish()
	require.ErrorIs(t, err, build.ErrOutput)
	require.False(t, IsUsage(err))
	require.Contains(t, err.Error(), appearedMsg)
	require.Equal(t, sneakBody, readFile(t, image))
	require.Equal(t, sneakBody, readFile(t, resources))
	require.Empty(t, leftoverTemps(t, dir))
}

// TestForceRollbackOrdering injects a failure after each of steps 2–5 and
// asserts the old pair is intact and every rollback step is reported.
func TestForceRollbackOrdering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		step    string
		want    []string
		notWant []string
	}{
		{
			step:    stepBakImage,
			want:    []string{rollRestoreImage},
			notWant: []string{rollRestoreResources, rollRemoveImage, rollRemoveResources},
		},
		{
			step: stepBakResources,
			want: []string{rollRestoreResources, rollRestoreImage},
		},
		{
			step: stepPublishResources,
			want: []string{rollRemoveResources, rollRestoreResources, rollRestoreImage},
		},
		{
			step: stepPublishImage,
			want: []string{rollRemoveImage, rollRemoveResources, rollRestoreResources, rollRestoreImage},
		},
	}

	for _, tt := range tests {
		t.Run("step "+tt.step, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			image := filepath.Join(dir, imageName)
			resources := filepath.Join(dir, resourcesName)
			writeFile(t, image, oldImageBody)
			writeFile(t, resources, oldResourcesBody)

			session := beginOffline(t, dir, true)
			writePayloads(t, session)
			session.failAfter = tt.step

			_, err := session.Publish()
			require.ErrorIs(t, err, build.ErrOutput)
			msg := err.Error()
			require.Contains(t, msg, "injected failure at step "+tt.step)
			for _, note := range tt.want {
				require.Contains(t, msg, note)
			}
			for _, note := range tt.notWant {
				require.NotContains(t, msg, note)
			}
			if strings.Contains(msg, rollRestoreResources) && strings.Contains(msg, rollRestoreImage) {
				require.Greater(t, strings.Index(msg, rollRestoreImage), strings.Index(msg, rollRestoreResources),
					"image.bak must be restored last")
			}

			require.Equal(t, oldImageBody, readFile(t, image))
			require.Equal(t, oldResourcesBody, readFile(t, resources))
			require.Empty(t, leftoverTemps(t, dir))
			require.Empty(t, leftoverBaks(t, dir))
			require.NotEqual(t, newImageBody, readFile(t, image))
			require.NotEqual(t, newResourcesBody, readFile(t, resources))
		})
	}
}

// TestConcurrentNonForceExactlyOneWins runs goroutines at one destination
// and asserts exactly one no-clobber claim succeeds.
func TestConcurrentNonForceExactlyOneWins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	image := filepath.Join(dir, imageName)
	start := make(chan struct{})
	var wins atomic.Int32
	var losses atomic.Int32
	var wg sync.WaitGroup
	wg.Add(concurrentWorkers)
	for range concurrentWorkers {
		go func() {
			defer wg.Done()
			session, err := Begin(Request{
				Image: image,
				Type:  build.ImageTypeISO,
			})
			if err != nil {
				losses.Add(1)
				return
			}
			defer session.Abort()
			_, _ = session.ImageWriter().Write([]byte(newImageBody))
			<-start
			_, err = session.Publish()
			if err == nil {
				wins.Add(1)
				return
			}
			if errors.Is(err, build.ErrOutput) && strings.Contains(err.Error(), appearedMsg) {
				losses.Add(1)
				return
			}
			losses.Add(1)
		}()
	}
	close(start)
	wg.Wait()

	require.Equal(t, int32(1), wins.Load(), "exactly one publisher must win the claim")
	require.Equal(t, int32(concurrentWorkers-1), losses.Load())
	require.Equal(t, newImageBody, readFile(t, image))
	require.Empty(t, leftoverTemps(t, dir))
}

// TestAbortCleansTemps removes unique temps and leaves no finals behind.
func TestAbortCleansTemps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	session := beginOffline(t, dir, false)
	writePayloads(t, session)
	require.NotEmpty(t, leftoverTemps(t, dir))
	session.Abort()
	require.Empty(t, leftoverTemps(t, dir))
	require.NoFileExists(t, session.Paths().Image)
	require.NoFileExists(t, session.Paths().Resources)
}

// TestImageDigestMatchesReread checks the hashing writer against a re-read
// of the published image bytes.
func TestImageDigestMatchesReread(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	session := beginOnline(t, dir)
	payload := []byte(newImageBody)
	n, err := session.ImageWriter().Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)

	pub, err := session.Publish()
	require.NoError(t, err)
	want := sha256Hex(payload)
	require.Equal(t, want, pub.ImageSHA256)
	require.Equal(t, want, sha256Hex([]byte(readFile(t, session.Paths().Image))))
	require.Empty(t, pub.ResourcesSHA256)
}

// TestResourcesDigestAfterInodeReplacement hashes by re-read after a
// WriteRescue-style unlink+create at the temp path, not a retained fd.
func TestResourcesDigestAfterInodeReplacement(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	session := beginOffline(t, dir, false)
	writePayloads(t, session)

	pub, err := session.Publish()
	require.NoError(t, err)
	require.Equal(t, sha256Hex([]byte(newImageBody)), pub.ImageSHA256)
	require.Equal(t, sha256Hex([]byte(newResourcesBody)), pub.ResourcesSHA256)
	require.Equal(t, pub.ResourcesSHA256, sha256Hex([]byte(readFile(t, session.Paths().Resources))))
	require.NotEqual(t, pub.ImageSHA256, pub.ResourcesSHA256)
}

// TestForceSuccessReportsBakLeftovers covers step 6: best-effort bak
// cleanup reports paths it could not remove.
func TestForceSuccessReportsBakLeftovers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	image := filepath.Join(dir, imageName)
	resources := filepath.Join(dir, resourcesName)
	writeFile(t, image, oldImageBody)
	writeFile(t, resources, oldResourcesBody)

	session := beginOffline(t, dir, true)
	writePayloads(t, session)
	session.remove = func(path string) error {
		if strings.HasSuffix(path, bakSuffix) {
			return errors.New("busy")
		}
		return os.Remove(path)
	}

	pub, err := session.Publish()
	require.NoError(t, err)
	require.Equal(t, newImageBody, readFile(t, image))
	require.Equal(t, newResourcesBody, readFile(t, resources))
	require.ElementsMatch(t, []string{image + bakSuffix, resources + bakSuffix}, pub.Leftovers)
}

// TestForceSuccessRemovesBaks deletes deterministic backups after commit.
func TestForceSuccessRemovesBaks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	image := filepath.Join(dir, imageName)
	resources := filepath.Join(dir, resourcesName)
	writeFile(t, image, oldImageBody)
	writeFile(t, resources, oldResourcesBody)

	session := beginOffline(t, dir, true)
	writePayloads(t, session)
	pub, err := session.Publish()
	require.NoError(t, err)
	require.Empty(t, pub.Leftovers)
	require.Equal(t, newImageBody, readFile(t, image))
	require.Equal(t, newResourcesBody, readFile(t, resources))
	require.Empty(t, leftoverBaks(t, dir))
	require.Empty(t, leftoverTemps(t, dir))
}

// TestConfirmTrueUsesForcePath replaces existing finals when confirm returns true.
func TestConfirmTrueUsesForcePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	image := filepath.Join(dir, imageName)
	writeFile(t, image, oldImageBody)
	session, err := Begin(Request{
		Image:   image,
		Type:    build.ImageTypeISO,
		Confirm: func() (bool, error) { return true, nil },
	})
	require.NoError(t, err)
	t.Cleanup(session.Abort)
	_, err = session.ImageWriter().Write([]byte(newImageBody))
	require.NoError(t, err)
	pub, err := session.Publish()
	require.NoError(t, err)
	require.Equal(t, newImageBody, readFile(t, image))
	require.Equal(t, sha256Hex([]byte(newImageBody)), pub.ImageSHA256)
}

// beginOnline starts a single-artifact session under dir.
func beginOnline(t *testing.T, dir string) *Session {
	t.Helper()
	session, err := Begin(Request{
		Image: filepath.Join(dir, imageName),
		Type:  build.ImageTypeISO,
	})
	require.NoError(t, err)
	t.Cleanup(session.Abort)
	return session
}

// beginOffline starts a two-artifact session under dir.
func beginOffline(t *testing.T, dir string, force bool) *Session {
	t.Helper()
	session, err := Begin(Request{
		Image:   filepath.Join(dir, imageName),
		Offline: true,
		Type:    build.ImageTypeISO,
		Force:   force,
	})
	require.NoError(t, err)
	t.Cleanup(session.Abort)
	return session
}

// writePayloads writes the image through the hashing writer and replaces the
// resources temp inode the way WriteRescue does (unlink + create).
func writePayloads(t *testing.T, session *Session) {
	t.Helper()
	_, err := session.ImageWriter().Write([]byte(newImageBody))
	require.NoError(t, err)
	replaceInode(t, session.ResourcesTemp(), newResourcesBody)
}

// replaceInode unlinks path and creates a new file at the same name.
func replaceInode(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.Remove(path))
	writeFile(t, path, body)
}

// writeFile writes body to path.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), filePerm))
}

// readFile returns the contents of path as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(body)
}

// leftoverTemps lists dot-prefixed *.tmp files in dir.
func leftoverTemps(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".*.tmp"))
	require.NoError(t, err)
	return matches
}

// leftoverBaks lists *.incusos-builder.bak files in dir.
func leftoverBaks(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*"+bakSuffix))
	require.NoError(t, err)
	return matches
}

// sha256Hex returns the lowercase hex digest of data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
