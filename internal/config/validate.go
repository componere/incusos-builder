package config

import (
	"fmt"
	"strings"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	fieldVersion                     = "version"
	fieldImageType                   = "image.type"
	fieldImageArchitecture           = "image.architecture"
	fieldSeedsApplications           = "seeds.applications"
	fieldSeedsSecurityRecoveryKeys   = "seeds.security.encryption_recovery_keys"
	fieldSeedsInstallTargetSortOrder = "seeds.install.target.sort_order"
	upstreamRecoveryKeysRejected     = "it is not possible to set encryption recovery key(s) via the security seed"
	sortOrderSmallest                = "smallest"
	sortOrderLargest                 = "largest"
	checkFrequencyNever              = "never"
)

// checkVersion requires version: 1. Unknown versions ask for a newer CLI.
func checkVersion(doc *document) error {
	if doc.Version == nil {
		return fmt.Errorf("%w: %s: required", ErrConfig, fieldVersion)
	}
	if *doc.Version != currentSchemaVersion {
		return fmt.Errorf("%w: %s: unsupported schema version; a newer CLI is required", ErrConfig, fieldVersion)
	}
	return nil
}

// applyDefaults fills channel, per-seed version, and offline update forcing.
func applyDefaults(doc *document) {
	if doc.Image.Channel == "" {
		doc.Image.Channel = build.DefaultChannel
	}
	defaultSeedVersions(&doc.Seeds)
	if !doc.Image.Offline {
		return
	}
	if doc.Seeds.Update == nil {
		doc.Seeds.Update = &apiseed.Update{Version: defaultSeedVersion}
	}
	doc.Seeds.Update.CheckFrequency = checkFrequencyNever
}

// defaultSeedVersions sets omitted per-seed version fields to "1".
func defaultSeedVersions(s *seeds) {
	for _, version := range presentSeedVersions(s) {
		setDefaultVersion(version)
	}
}

// presentSeedVersions returns Version fields of every non-nil seed section.
func presentSeedVersions(s *seeds) []*string {
	var out []*string
	if s.Applications != nil {
		out = append(out, &s.Applications.Version)
	}
	if s.Incus != nil {
		out = append(out, &s.Incus.Version)
	}
	if s.Install != nil {
		out = append(out, &s.Install.Version)
	}
	if s.MigrationManager != nil {
		out = append(out, &s.MigrationManager.Version)
	}
	if s.Network != nil {
		out = append(out, &s.Network.Version)
	}
	if s.OperationsCenter != nil {
		out = append(out, &s.OperationsCenter.Version)
	}
	if s.Provider != nil {
		out = append(out, &s.Provider.Version)
	}
	if s.Services != nil {
		out = append(out, &s.Services.Version)
	}
	if s.Update != nil {
		out = append(out, &s.Update.Version)
	}
	if s.Kernel != nil {
		out = append(out, &s.Kernel.Version)
	}
	if s.Security != nil {
		out = append(out, &s.Security.Version)
	}
	return out
}

// setDefaultVersion fills an omitted seed version with "1".
func setDefaultVersion(version *string) {
	if *version == "" {
		*version = defaultSeedVersion
	}
}

// validate runs §4 checks. All errors wrap [ErrConfig] and name field paths.
func validate(doc *document) error {
	if err := validateImage(doc.Image); err != nil {
		return err
	}
	if err := validateInstall(doc.Seeds.Install); err != nil {
		return err
	}
	if err := validateSecurity(doc.Seeds.Security); err != nil {
		return err
	}
	return validateOffline(doc)
}

// validateImage checks type and architecture enums.
func validateImage(img image) error {
	switch img.Type {
	case build.ImageTypeISO, build.ImageTypeRaw:
	default:
		return fmt.Errorf("%w: %s: must be iso or raw", ErrConfig, fieldImageType)
	}
	switch img.Architecture {
	case build.ArchX8664, build.ArchAarch64:
	default:
		return fmt.Errorf("%w: %s: must be x86_64 or aarch64", ErrConfig, fieldImageArchitecture)
	}
	return nil
}

// validateInstall checks install.target.sort_order against upstream InstallTarget.
func validateInstall(install *apiseed.Install) error {
	if install == nil || install.Target == nil {
		return nil
	}
	switch strings.ToLower(install.Target.SortOrder) {
	case "", sortOrderSmallest, sortOrderLargest:
		return nil
	default:
		return fmt.Errorf("%w: %s: must be empty, smallest, or largest", ErrConfig, fieldSeedsInstallTargetSortOrder)
	}
}

// validateSecurity rejects non-empty encryption_recovery_keys.
func validateSecurity(sec *apiseed.Security) error {
	if sec == nil || len(sec.EncryptionRecoveryKeys) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s: %s; see %s",
		ErrConfig, fieldSeedsSecurityRecoveryKeys, upstreamRecoveryKeysRejected, diskEncryptionDocs)
}

// validateOffline applies the resources-media applications requirement.
func validateOffline(doc *document) error {
	if !doc.Image.Offline {
		return nil
	}
	if doc.Seeds.Applications == nil || len(doc.Seeds.Applications.Applications) == 0 {
		return fmt.Errorf("%w: %s: required when image.offline is true", ErrConfig, fieldSeedsApplications)
	}
	return nil
}
