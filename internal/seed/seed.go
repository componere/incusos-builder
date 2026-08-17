package seed

import (
	"archive/tar"
	"bytes"
	"io"

	"go.yaml.in/yaml/v4"

	"github.com/componere/incusos-builder/internal/build"
)

// seedTarMode is the tar header mode for every seed file, matching writeSeed.
const seedTarMode = 0o600

// Render serializes s into an uncompressed tar of YAML seed files.
//
// Non-nil sections become one entry each. The nine web-customizer sections
// use writeSeed's names and order; kernel and security follow as
// kernel.yaml and security.yaml. Empty Seeds yields a valid zero-entry tar.
// The returned size is the tar length in bytes, including end-of-archive
// blocks.
func Render(s build.Seeds) ([]byte, int64, error) {
	archiveContents, err := collectSeedYAML(s)
	if err != nil {
		return nil, 0, err
	}
	return writeSeedTar(archiveContents)
}

// collectSeedYAML serializes each non-nil seed section in archive order.
func collectSeedYAML(s build.Seeds) ([][]string, error) {
	archive := [][]string{}
	if err := appendCustomizerSeeds(&archive, s); err != nil {
		return nil, err
	}
	if err := appendCLISeeds(&archive, s); err != nil {
		return nil, err
	}
	return archive, nil
}

// appendCustomizerSeeds serializes the nine web-customizer sections in
// archive order.
func appendCustomizerSeeds(archive *[][]string, s build.Seeds) error {
	if err := appendIfPresent(archive, "applications.yaml", s.Applications); err != nil {
		return err
	}
	if err := appendIfPresent(archive, "incus.yaml", s.Incus); err != nil {
		return err
	}
	if err := appendIfPresent(archive, "operations-center.yaml", s.OperationsCenter); err != nil {
		return err
	}
	if err := appendIfPresent(archive, "migration-manager.yaml", s.MigrationManager); err != nil {
		return err
	}
	if err := appendIfPresent(archive, "install.yaml", s.Install); err != nil {
		return err
	}
	if err := appendIfPresent(archive, "network.yaml", s.Network); err != nil {
		return err
	}
	if err := appendIfPresent(archive, "provider.yaml", s.Provider); err != nil {
		return err
	}
	if err := appendIfPresent(archive, "services.yaml", s.Services); err != nil {
		return err
	}
	return appendIfPresent(archive, "update.yaml", s.Update)
}

// appendCLISeeds serializes kernel.yaml and security.yaml after the
// customizer nine.
func appendCLISeeds(archive *[][]string, s build.Seeds) error {
	if err := appendIfPresent(archive, "kernel.yaml", s.Kernel); err != nil {
		return err
	}
	return appendIfPresent(archive, "security.yaml", s.Security)
}

// appendIfPresent serializes v into archive under name when v is non-nil.
func appendIfPresent[T any](archive *[][]string, name string, v *T) error {
	if v == nil {
		return nil
	}
	return appendYAML(archive, name, v)
}

// appendYAML dumps v with yaml.WithV2Defaults and records a tar member.
func appendYAML(archive *[][]string, name string, v any) error {
	yamlContents, err := yaml.Dump(v, yaml.WithV2Defaults())
	if err != nil {
		return err
	}
	*archive = append(*archive, []string{name, string(yamlContents)})
	return nil
}

// writeSeedTar packs files into an uncompressed tar. Headers set Name,
// Mode 0600, and Size only.
func writeSeedTar(files [][]string) ([]byte, int64, error) {
	var buf bytes.Buffer
	wc := &writeCounter{}
	tw := tar.NewWriter(io.MultiWriter(wc, &buf))
	for _, file := range files {
		hdr := &tar.Header{
			Name: file[0],
			Mode: seedTarMode,
			Size: int64(len(file[1])),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, 0, err
		}
		if _, err := tw.Write([]byte(file[1])); err != nil {
			return nil, 0, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), int64(wc.size), nil
}

// writeCounter counts bytes written through the tar writer, including
// end-of-archive blocks.
type writeCounter struct {
	// size is the running byte count, including end-of-archive blocks.
	size int
}

// Write implements [io.Writer].
func (wc *writeCounter) Write(buf []byte) (int, error) {
	size := len(buf)
	wc.size += size
	return size, nil
}
