package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	// bakSuffix is appended to a final path to name its --force backup.
	// Names are deterministic so a crash is a documented rename recovery.
	bakSuffix = ".incusos-builder.bak"
	// appearedMsg is the exit-6 wording when a no-clobber claim hits EEXIST.
	appearedMsg = "output appeared during the build; re-run with --force"
	// stdoutSentinel is the reserved stream path rejected by the publisher.
	// Streaming -o - is a command concern, not a publication concern.
	stdoutSentinel = "-"
	// extISO is the resources-media extension for iso builds.
	extISO = "iso"
	// extIMG is the resources-media extension for raw builds.
	extIMG = "img"
	// resourcesMiddle is the default resources filename infix.
	resourcesMiddle = ".resources."
	// claimMode is the mode used for O_CREAT|O_EXCL name claims.
	claimMode = 0o644
	// stepBakImage is architecture §3 force step 2.
	stepBakImage = "2"
	// stepBakResources is architecture §3 force step 3.
	stepBakResources = "3"
	// stepPublishResources is architecture §3 force step 4.
	stepPublishResources = "4"
	// stepPublishImage is architecture §3 force step 5.
	stepPublishImage = "5"
	// rollRemoveImage is a rollback note for a published/claimed image.
	rollRemoveImage = "removed new image"
	// rollRemoveResources is a rollback note for a published/claimed resources file.
	rollRemoveResources = "removed new resources"
	// rollRestoreResources is a rollback note for restoring resources.bak.
	rollRestoreResources = "restored resources"
	// rollRestoreImage is a rollback note for restoring image.bak (always last).
	rollRestoreImage = "restored image"
	// leftoverLabel prefixes leftover paths in fail() error text.
	leftoverLabel = "leftover"
)

// ErrUsage marks a CLI usage error. Root maps it to process exit code 2.
var ErrUsage = errors.New("usage error")

// ConfirmFunc is injected by the command to authorize replacing existing
// finals. The publisher never prompts. A nil value refuses. Returning true
// selects the --force publication path (both-or-nothing with .bak files).
type ConfirmFunc func() (bool, error)

// Request is the publisher input derived from CLI flags and the build spec.
type Request struct {
	// Image is the -o destination. "-" is a usage error here.
	Image string
	// Resources is --resources-output. Empty means default when Offline.
	// A non-empty value on an online request is a usage error.
	Resources string
	// Offline selects the two-artifact lifecycle.
	Offline bool
	// Type selects the default resources extension (iso → iso, raw → img).
	Type build.ImageType
	// Force authorizes replacement of existing finals.
	Force bool
	// Confirm is called when a final exists and Force is false.
	Confirm ConfirmFunc
}

// Paths holds cleaned final destinations. Resources is empty for online builds.
type Paths struct {
	// Image is the cleaned -o path.
	Image string
	// Resources is the cleaned resources path. Empty when online.
	Resources string
}

// Publication is the successful result of [Session.Publish].
type Publication struct {
	// ImageSHA256 is the lowercase hex digest of the bytes at Paths.Image.
	ImageSHA256 string
	// ResourcesSHA256 is the lowercase hex digest of the bytes at
	// Paths.Resources. Empty when the session is online.
	ResourcesSHA256 string
	// Leftovers are .incusos-builder.bak paths that best-effort cleanup
	// could not remove. They are harmless; recovery is a rename.
	Leftovers []string
}

// Session is one in-progress publication: unique temps, a hashing image
// writer, claim-then-rename, and --force rollback.
type Session struct {
	// image is the OS-image artifact.
	image artifact
	// resources is the rescue-media artifact. Zero when online.
	resources artifact
	// hasher wraps the image temp with a running SHA-256.
	hasher *hashingWriter
	// replace selects the --force bak/restore path.
	replace bool
	// rollback records restore steps taken on a handled failure.
	rollback []string
	// leftovers records paths that could not be removed during cleanup.
	leftovers []string
	// complete is true after a successful Publish; Abort becomes a no-op.
	complete bool
	// cleaned is true after Abort/failure cleanup has run.
	cleaned bool
	// failAfter, if non-empty, injects an error after that §3 step ("2"–"5").
	failAfter string
	// rename moves a path; tests inject failures. Nil means [os.Rename].
	rename func(string, string) error
	// remove deletes a path; tests inject leftovers. Nil means [os.Remove].
	remove func(string) error
}

// artifact is one published file (image or resources) plus its temp and bak.
type artifact struct {
	// final is the cleaned destination path.
	final string
	// temp is the exclusive CreateTemp path. Empty after a successful rename.
	temp string
	// bak is final + bakSuffix.
	bak string
	// file is the open image temp. Nil for resources (inode may be replaced).
	file *os.File
	// claimed is true after O_CREAT|O_EXCL succeeded and before rename.
	claimed bool
	// published is true after rename(temp, final) succeeded.
	published bool
	// backedUp is true after rename(final, bak) succeeded this run.
	backedUp bool
}

// hashingWriter writes to w while hashing the exact bytes w accepted.
type hashingWriter struct {
	// w is the image temp file.
	w io.Writer
	// h is the running SHA-256.
	h hash.Hash
}

// IsUsage reports whether err is (or wraps) [ErrUsage].
func IsUsage(err error) bool {
	return errors.Is(err, ErrUsage)
}

// Begin validates paths, applies the overwrite policy, and creates unique
// temps. The caller writes the image through [Session.ImageWriter] and
// rescue media into [Session.ResourcesTemp], then calls [Session.Publish]
// or [Session.Abort].
func Begin(req Request) (*Session, error) {
	paths, err := resolvePaths(req.Image, req.Resources, req.Offline, req.Type)
	if err != nil {
		return nil, err
	}
	replace, err := authorize(paths, req)
	if err != nil {
		return nil, err
	}
	session := &Session{
		image: artifact{
			final: paths.Image,
			bak:   paths.Image + bakSuffix,
		},
		replace: replace,
		rename:  os.Rename,
		remove:  os.Remove,
	}
	if paths.Resources != "" {
		session.resources = artifact{
			final: paths.Resources,
			bak:   paths.Resources + bakSuffix,
		}
	}
	if err := session.createTemps(); err != nil {
		session.Abort()
		return nil, err
	}
	return session, nil
}

// ImageWriter returns the SHA-256 hashing writer over the image temp.
// Digests cover the exact bytes accepted by the temp file.
func (s *Session) ImageWriter() io.Writer {
	return s.hasher
}

// ResourcesTemp returns the exclusive temp path for [build.RescueWriter].
// Empty when the session is online. The media adapter replaces this inode;
// callers must not retain a file descriptor across WriteRescue. Hashing
// re-opens the path after the write.
func (s *Session) ResourcesTemp() string {
	return s.resources.temp
}

// Paths returns the cleaned final destinations.
func (s *Session) Paths() Paths {
	return Paths{Image: s.image.final, Resources: s.resources.final}
}

// Publish fsyncs and hashes both temps, then claim-then-renames (resources
// first, image last). With replace set, it follows the §3 six-step --force
// ordering and reverse-order rollback on handled failure. Temps and claims
// are removed on failure; nothing partial is left at a final path.
func (s *Session) Publish() (Publication, error) {
	imageDigest, err := s.finishImageTemp()
	if err != nil {
		return Publication{}, s.fail(err)
	}
	var resourcesDigest string
	if s.resources.temp != "" {
		resourcesDigest, err = s.finishResourcesTemp()
		if err != nil {
			return Publication{}, s.fail(err)
		}
	}
	if s.replace {
		err = s.publishForce()
	} else {
		err = s.publishNoClobber()
	}
	if err != nil {
		return Publication{}, s.fail(err)
	}
	leftovers := s.removeBaks()
	s.complete = true
	s.cleaned = true
	return Publication{
		ImageSHA256:     imageDigest,
		ResourcesSHA256: resourcesDigest,
		Leftovers:       leftovers,
	}, nil
}

// Abort removes temps and claims and restores any .bak files. It is a
// no-op after a successful [Publish] and is safe to defer.
func (s *Session) Abort() {
	if s == nil {
		return
	}
	s.cleanup()
}

// Write hashes the bytes actually accepted by the image temp.
func (w *hashingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if n > 0 {
		_, _ = w.h.Write(p[:n])
	}
	return n, err
}

// resolvePaths defaults, cleans, and validates final destinations.
func resolvePaths(image, resources string, offline bool, typ build.ImageType) (Paths, error) {
	if strings.TrimSpace(image) == "" {
		return Paths{}, usagef("output path is required")
	}
	if isStdout(image) {
		return Paths{}, usagef("output path cannot be %s", stdoutSentinel)
	}
	if offline && strings.TrimSpace(resources) == "" {
		ext, err := resourcesExt(typ)
		if err != nil {
			return Paths{}, err
		}
		resources = defaultResources(image, ext)
	}
	if !offline {
		if strings.TrimSpace(resources) != "" {
			return Paths{}, usagef("--resources-output requires offline: true in the config")
		}
		resources = ""
	}
	image = filepath.Clean(image)
	if resources != "" {
		if isStdout(resources) {
			return Paths{}, usagef("resources path cannot be %s", stdoutSentinel)
		}
		resources = filepath.Clean(resources)
	}
	if isStdout(image) {
		return Paths{}, usagef("output path cannot be %s", stdoutSentinel)
	}
	if resources != "" && image == resources {
		return Paths{}, usagef("image and resources paths must be distinct")
	}
	return Paths{Image: image, Resources: resources}, nil
}

// defaultResources returns <out-stem>.resources.<iso|img> beside image.
func defaultResources(image, ext string) string {
	base := filepath.Base(image)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(filepath.Dir(image), stem+resourcesMiddle+ext)
}

// resourcesExt maps the image type onto the resources-media file extension.
func resourcesExt(typ build.ImageType) (string, error) {
	switch typ {
	case build.ImageTypeISO:
		return extISO, nil
	case build.ImageTypeRaw:
		return extIMG, nil
	default:
		return "", usagef("unknown image type %q", typ)
	}
}

// isStdout reports whether path is the reserved "-" stream sentinel.
func isStdout(path string) bool {
	return path == stdoutSentinel || filepath.Clean(path) == stdoutSentinel
}

// authorize is the pre-work existence check. It is UX, not enforcement:
// a file appearing later is refused by O_EXCL, never overwritten, unless
// replacement was authorized (Force or Confirm).
func authorize(paths Paths, req Request) (bool, error) {
	existing, err := existingFinals(paths)
	if err != nil {
		return false, err
	}
	if req.Force {
		return true, nil
	}
	if len(existing) == 0 {
		return false, nil
	}
	if req.Confirm == nil {
		return false, refuseOverwrite(existing)
	}
	ok, err := req.Confirm()
	if err != nil {
		if IsUsage(err) {
			return false, err
		}
		return false, outputWrap(err, "confirm overwrite")
	}
	if !ok {
		return false, refuseOverwrite(existing)
	}
	return true, nil
}

// existingFinals lists finals that already occupy the destination names.
func existingFinals(paths Paths) ([]string, error) {
	var existing []string
	for _, path := range []string{paths.Image, paths.Resources} {
		if path == "" {
			continue
		}
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, outputWrap(err, "stat %s", path)
		}
		existing = append(existing, path)
	}
	return existing, nil
}

// refuseOverwrite is the exit-2 wording when replacement was not authorized.
func refuseOverwrite(existing []string) error {
	return usagef("refusing to overwrite %s; re-run with --force", strings.Join(existing, ", "))
}

// createTemps makes one exclusive temp per artifact in its destination directory.
func (s *Session) createTemps() error {
	img, err := createTemp(s.image.final)
	if err != nil {
		return err
	}
	s.image.temp = img.Name()
	s.image.file = img
	s.hasher = &hashingWriter{w: img, h: sha256.New()}
	if s.resources.final == "" {
		return nil
	}
	res, err := createTemp(s.resources.final)
	if err != nil {
		return err
	}
	s.resources.temp = res.Name()
	if err := res.Close(); err != nil {
		return outputWrap(err, "close resources temp")
	}
	return nil
}

// createTemp creates [os.CreateTemp](dir(dest), ".<base>-*.tmp") and
// chmods it to [claimMode] so rename publishes a world-readable artifact.
func createTemp(dest string) (*os.File, error) {
	dir := filepath.Dir(dest)
	base := filepath.Base(dest)
	f, err := os.CreateTemp(dir, "."+base+"-*.tmp")
	if err != nil {
		return nil, outputWrap(err, "create temp for %s", dest)
	}
	if err := f.Chmod(claimMode); err != nil {
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		return nil, outputWrap(err, "chmod temp for %s", dest)
	}
	return f, nil
}

// finishImageTemp fsyncs and closes the image temp and returns its digest.
func (s *Session) finishImageTemp() (string, error) {
	if s.image.file == nil || s.hasher == nil {
		return "", outputf("image temp is not open")
	}
	if err := s.image.file.Sync(); err != nil {
		return "", outputWrap(err, "fsync image temp")
	}
	if err := s.image.file.Close(); err != nil {
		s.image.file = nil
		return "", outputWrap(err, "close image temp")
	}
	s.image.file = nil
	return hex.EncodeToString(s.hasher.h.Sum(nil)), nil
}

// finishResourcesTemp fsyncs via a fresh open, then hashes by sequential
// re-read. A retained fd would refer to the unlinked CreateTemp placeholder
// after [build.RescueWriter] replaces the inode.
func (s *Session) finishResourcesTemp() (string, error) {
	if err := fsyncPath(s.resources.temp); err != nil {
		return "", err
	}
	return hashPath(s.resources.temp)
}

// fsyncPath opens path freshly, fsyncs, and closes.
func fsyncPath(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return outputWrap(err, "open %s for fsync", path)
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if syncErr != nil {
		return outputWrap(syncErr, "fsync %s", path)
	}
	if closeErr != nil {
		return outputWrap(closeErr, "close %s after fsync", path)
	}
	return nil
}

// hashPath sequentially re-reads path and returns its lowercase hex SHA-256.
func hashPath(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", outputWrap(err, "open %s for hash", path)
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, f)
	closeErr := f.Close()
	if copyErr != nil {
		return "", outputWrap(copyErr, "hash %s", path)
	}
	if closeErr != nil {
		return "", outputWrap(closeErr, "close %s after hash", path)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// publishNoClobber claim-then-renames resources first, then image.
func (s *Session) publishNoClobber() error {
	if err := s.claimThenRename(&s.resources); err != nil {
		return err
	}
	return s.claimThenRename(&s.image)
}

// publishForce runs architecture §3 steps 2–5 (step 1 is finish+hash).
func (s *Session) publishForce() error {
	if err := s.bak(&s.image); err != nil {
		return err
	}
	if err := s.afterStep(stepBakImage); err != nil {
		return err
	}
	if err := s.bak(&s.resources); err != nil {
		return err
	}
	if err := s.afterStep(stepBakResources); err != nil {
		return err
	}
	if err := s.claimThenRename(&s.resources); err != nil {
		return err
	}
	if err := s.afterStep(stepPublishResources); err != nil {
		return err
	}
	if err := s.claimThenRename(&s.image); err != nil {
		return err
	}
	return s.afterStep(stepPublishImage)
}

// bak moves final aside to a deterministic .incusos-builder.bak name.
// Missing finals are skipped. A stale same-named bak is replaced.
func (s *Session) bak(a *artifact) error {
	if a.final == "" {
		return nil
	}
	_, err := os.Stat(a.final)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return outputWrap(err, "stat %s", a.final)
	}
	if err := s.rename(a.final, a.bak); err != nil {
		return outputWrap(err, "rename %s to %s", a.final, a.bak)
	}
	a.backedUp = true
	return nil
}

// claimThenRename atomically claims final with O_CREAT|O_EXCL, then renames
// the complete temp over that claim. EEXIST is the race-free no-clobber miss.
func (s *Session) claimThenRename(a *artifact) error {
	if a.final == "" || a.temp == "" {
		return nil
	}
	f, err := os.OpenFile(a.final, os.O_CREATE|os.O_EXCL, claimMode)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", build.ErrOutput, appearedMsg)
		}
		return outputWrap(err, "claim %s", a.final)
	}
	a.claimed = true
	if err := f.Close(); err != nil {
		return outputWrap(err, "close claim %s", a.final)
	}
	if err := s.rename(a.temp, a.final); err != nil {
		return outputWrap(err, "publish %s", a.final)
	}
	a.claimed = false
	a.published = true
	a.temp = ""
	return nil
}

// afterStep injects a test failure after a completed --force step.
func (s *Session) afterStep(step string) error {
	if s.failAfter == step {
		return outputf("injected failure at step %s", step)
	}
	return nil
}

// removeBaks best-effort deletes backups created this run. Leftovers are
// reported and harmless.
func (s *Session) removeBaks() []string {
	var leftovers []string
	for _, a := range []*artifact{&s.image, &s.resources} {
		if !a.backedUp {
			continue
		}
		err := s.remove(a.bak)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			leftovers = append(leftovers, a.bak)
			continue
		}
		a.backedUp = false
	}
	return leftovers
}

// fail runs abort cleanup and attaches every rollback step and leftover
// path to err. Leftovers are the failure-path report channel; they are
// not returned as Publication.Leftovers.
func (s *Session) fail(err error) error {
	s.cleanup()
	parts := make([]string, 0, len(s.rollback)+1)
	parts = append(parts, s.rollback...)
	if len(s.leftovers) > 0 {
		parts = append(parts, leftoverLabel+" "+strings.Join(s.leftovers, ", "))
	}
	if len(parts) == 0 {
		return err
	}
	return fmt.Errorf("%w (%s)", err, strings.Join(parts, "; "))
}

// cleanup restores the old pair, removes claims/temps, and is idempotent.
func (s *Session) cleanup() {
	if s.complete || s.cleaned {
		return
	}
	s.cleaned = true
	s.closeImage()
	s.rollbackPublication()
	s.removeTemp(s.image.temp)
	s.removeTemp(s.resources.temp)
}

// closeImage closes the image temp if it is still open.
func (s *Session) closeImage() {
	if s.image.file == nil {
		return
	}
	_ = s.image.file.Close()
	s.image.file = nil
}

// rollbackPublication undoes steps 2–5 in reverse: drop new finals, restore
// resources.bak, restore image.bak last.
func (s *Session) rollbackPublication() {
	s.unpublish(&s.image, rollRemoveImage)
	s.unpublish(&s.resources, rollRemoveResources)
	s.restore(&s.resources, rollRestoreResources)
	s.restore(&s.image, rollRestoreImage)
}

// unpublish removes a claimed or published final and records the step.
func (s *Session) unpublish(a *artifact, verb string) {
	if a.final == "" || (!a.published && !a.claimed) {
		return
	}
	err := s.remove(a.final)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.leftovers = append(s.leftovers, a.final)
		s.rollback = append(s.rollback, verb+" "+a.final+" failed: "+err.Error())
		return
	}
	s.rollback = append(s.rollback, verb+" "+a.final)
	a.published = false
	a.claimed = false
}

// restore moves bak back onto final and records the step.
func (s *Session) restore(a *artifact, verb string) {
	if a.final == "" || !a.backedUp {
		return
	}
	if err := s.rename(a.bak, a.final); err != nil {
		s.leftovers = append(s.leftovers, a.bak)
		s.rollback = append(s.rollback, verb+" "+a.final+" failed: "+err.Error())
		return
	}
	s.rollback = append(s.rollback, verb+" "+a.final)
	a.backedUp = false
}

// removeTemp deletes a leftover temp, ignoring absence.
func (s *Session) removeTemp(path string) {
	if path == "" {
		return
	}
	err := s.remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.leftovers = append(s.leftovers, path)
	}
}

// usagef wraps [ErrUsage] with a formatted message.
func usagef(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrUsage}, args...)...)
}

// outputf wraps [build.ErrOutput] with a formatted message.
func outputf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{build.ErrOutput}, args...)...)
}

// outputWrap wraps err as [build.ErrOutput] unless it already is one or [ErrUsage].
func outputWrap(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	if IsUsage(err) || errors.Is(err, build.ErrOutput) {
		return fmt.Errorf(format+": %w", append(args, err)...)
	}
	return fmt.Errorf("%w: "+format+": %w", append([]any{build.ErrOutput}, append(args, err)...)...)
}
