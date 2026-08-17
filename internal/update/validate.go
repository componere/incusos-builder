package update

import (
	"fmt"
	"strings"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

const (
	sha256HexLen = 64
	giB          = 1 << 30
	// maxAssetSize is the untrusted UpdateFile.Size sanity bound of 8 GiB.
	maxAssetSize = 8 * giB
	tamperSuffix = "untrusted metadata; possible tampering"
)

// ValidateVersion returns an error unless version is a legal untrusted
// UpdateFull.Version value. It must be non-empty, match [A-Za-z0-9._-]+,
// and must not be "." or "..". Callers must invoke this before using
// version as a URL or filesystem path.
func ValidateVersion(version string) error {
	if !validIdent(version) {
		return fmt.Errorf("%w: version %q rejected; %s", ErrFetch, version, tamperSuffix)
	}
	return nil
}

// ValidateFilename returns an error unless filename is a legal untrusted
// UpdateFile.Filename value. It must be a non-empty relative path whose
// '/' segments each pass the same allowlist as [ValidateVersion], with
// empty, ".", and ".." segments rejected. Callers must invoke this
// before using filename as a URL or filesystem path.
func ValidateFilename(filename string) error {
	if filename == "" || strings.HasPrefix(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("%w: filename %q rejected; %s", ErrFetch, filename, tamperSuffix)
	}
	for segment := range strings.SplitSeq(filename, "/") {
		if !validIdent(segment) {
			return fmt.Errorf("%w: filename %q rejected; %s", ErrFetch, filename, tamperSuffix)
		}
	}
	return nil
}

// ValidateSHA256 returns an error unless digest is exactly 64 lowercase
// hex characters. Callers must invoke this before using the digest as a
// cache path component.
func ValidateSHA256(digest string) error {
	if len(digest) != sha256HexLen || !allLowerHex(digest) {
		return fmt.Errorf("%w: sha256 %q rejected; %s", ErrFetch, digest, tamperSuffix)
	}
	return nil
}

// validateSize enforces 0 < Size ≤ 8 GiB on untrusted UpdateFile.Size.
func validateSize(size int64) error {
	if size <= 0 || size > maxAssetSize {
		return fmt.Errorf("%w: size %d rejected; %s", ErrFetch, size, tamperSuffix)
	}
	return nil
}

// validateAsset runs every untrusted-field check required before URL or
// filesystem use of (version, file).
func validateAsset(version string, file apiimages.UpdateFile) error {
	if err := ValidateVersion(version); err != nil {
		return err
	}
	if err := ValidateFilename(file.Filename); err != nil {
		return err
	}
	if err := ValidateSHA256(file.Sha256); err != nil {
		return err
	}
	return validateSize(file.Size)
}

// validIdent reports whether s is a non-empty allowlisted identifier.
func validIdent(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		if !isIdentRune(r) {
			return false
		}
	}
	return true
}

// isIdentRune reports whether r is in [A-Za-z0-9._-].
func isIdentRune(r rune) bool {
	return r == '-' || r == '.' || r == '_' ||
		(r >= '0' && r <= '9') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z')
}

// allLowerHex reports whether s is entirely lowercase hexadecimal.
func allLowerHex(s string) bool {
	for i := range len(s) {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
