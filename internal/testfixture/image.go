package testfixture

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

const (
	osImageName   = ArchX8664 + "/IncusOS_" + Version + ".img.gz"
	zeroChunkSize = 1 << 20
	gzipOSByte    = 255
)

// writeOSImage streams a sparse gzip GPT image to
// <dir>/<version>/<osImageName> and returns its UpdateFile plus the
// decompressed size. The decompressed stream is never buffered as a whole.
func writeOSImage(dir string) (apiimages.UpdateFile, int64, error) {
	layout := fixtureLayout()
	path := filepath.Join(versionDir(dir), filepath.FromSlash(osImageName))
	err := os.MkdirAll(filepath.Dir(path), dirPerm)
	if err != nil {
		return apiimages.UpdateFile{}, 0, fmt.Errorf("create image directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, filePerm)
	if err != nil {
		return apiimages.UpdateFile{}, 0, fmt.Errorf("create OS image: %w", err)
	}

	sum := sha256.New()
	counted := &countWriter{w: io.MultiWriter(file, sum)}
	err = gzipSparseImage(counted, layout)
	closeErr := file.Close()
	if err != nil {
		return apiimages.UpdateFile{}, 0, err
	}
	if closeErr != nil {
		return apiimages.UpdateFile{}, 0, fmt.Errorf("close OS image: %w", closeErr)
	}

	fileInfo := apiimages.UpdateFile{
		Architecture: apiimages.UpdateFileArchitecture64BitX86,
		Component:    apiimages.UpdateFileComponentOS,
		Filename:     osImageName,
		Sha256:       hex.EncodeToString(sum.Sum(nil)),
		Size:         counted.n,
		Type:         apiimages.UpdateFileTypeImageRaw,
	}

	return fileInfo, layout.TotalBytes, nil
}

// gzipSparseImage writes a gzip stream whose decompressed payload is a GPT
// disk of layout.TotalBytes: primary GPT, a zeros run through seed-data,
// then the backup GPT tail.
func gzipSparseImage(w io.Writer, layout diskLayout) error {
	zw, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("gzip writer: %w", err)
	}
	zw.OS = gzipOSByte

	primary := gptPrimary(layout)
	_, err = zw.Write(primary)
	if err != nil {
		_ = zw.Close()

		return fmt.Errorf("write primary GPT: %w", err)
	}

	zeros := layout.TotalBytes - int64(len(primary)) - int64(gptEntryArrayLBAs+1)*sectorSize
	err = writeZeros(zw, zeros)
	if err != nil {
		_ = zw.Close()

		return err
	}

	_, err = zw.Write(gptBackupEntries(layout))
	if err != nil {
		_ = zw.Close()

		return fmt.Errorf("write backup GPT entries: %w", err)
	}

	_, err = zw.Write(gptBackupHeader(layout))
	if err != nil {
		_ = zw.Close()

		return fmt.Errorf("write backup GPT header: %w", err)
	}

	err = zw.Close()
	if err != nil {
		return fmt.Errorf("close gzip OS image: %w", err)
	}

	return nil
}

// writeZeros writes n zero bytes to w in [zeroChunkSize] chunks.
func writeZeros(w io.Writer, n int64) error {
	if n < 0 {
		return fmt.Errorf("write zeros: negative length %d", n)
	}

	buf := make([]byte, zeroChunkSize)
	for n > 0 {
		chunk := min(int64(len(buf)), n)
		nw, err := w.Write(buf[:chunk])
		if err != nil {
			return fmt.Errorf("write zero run: %w", err)
		}
		n -= int64(nw)
	}

	return nil
}

// writeApps writes 1–2 tiny application gzip assets and returns their
// UpdateFile entries.
func writeApps(dir string) ([]apiimages.UpdateFile, error) {
	specs := []struct {
		filename  string
		component apiimages.UpdateFileComponent
		payload   []byte
	}{
		{
			filename:  ArchX8664 + "/debug.raw.gz",
			component: apiimages.UpdateFileComponentDebug,
			payload:   []byte("fixture-debug-application"),
		},
		{
			filename:  ArchX8664 + "/incus.raw.gz",
			component: apiimages.UpdateFileComponentIncus,
			payload:   []byte("fixture-incus-application"),
		},
	}

	files := make([]apiimages.UpdateFile, 0, len(specs))
	for _, spec := range specs {
		file, err := writeGzipAsset(dir, spec.filename, spec.component, spec.payload)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	return files, nil
}

// writeGzipAsset gzip-compresses payload into <dir>/<version>/<filename>.
func writeGzipAsset(
	dir, filename string,
	component apiimages.UpdateFileComponent,
	payload []byte,
) (apiimages.UpdateFile, error) {
	path := filepath.Join(versionDir(dir), filepath.FromSlash(filename))
	err := os.MkdirAll(filepath.Dir(path), dirPerm)
	if err != nil {
		return apiimages.UpdateFile{}, fmt.Errorf("create asset directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, filePerm)
	if err != nil {
		return apiimages.UpdateFile{}, fmt.Errorf("create asset %s: %w", filename, err)
	}

	sum := sha256.New()
	counted := &countWriter{w: io.MultiWriter(file, sum)}
	zw, err := gzip.NewWriterLevel(counted, gzip.BestSpeed)
	if err != nil {
		_ = file.Close()

		return apiimages.UpdateFile{}, fmt.Errorf("gzip writer: %w", err)
	}
	zw.OS = gzipOSByte
	_, err = zw.Write(payload)
	if err != nil {
		_ = zw.Close()
		_ = file.Close()

		return apiimages.UpdateFile{}, fmt.Errorf("write asset %s: %w", filename, err)
	}
	err = zw.Close()
	closeErr := file.Close()
	if err != nil {
		return apiimages.UpdateFile{}, fmt.Errorf("close gzip %s: %w", filename, err)
	}
	if closeErr != nil {
		return apiimages.UpdateFile{}, fmt.Errorf("close asset %s: %w", filename, closeErr)
	}

	return apiimages.UpdateFile{
		Architecture: apiimages.UpdateFileArchitecture64BitX86,
		Component:    component,
		Filename:     filename,
		Sha256:       hex.EncodeToString(sum.Sum(nil)),
		Size:         counted.n,
		Type:         apiimages.UpdateFileTypeApplication,
	}, nil
}

// countWriter counts bytes written to w.
type countWriter struct {
	// w is the underlying writer.
	w io.Writer
	// n is the number of bytes accepted.
	n int64
}

// Write forwards p to the underlying writer and counts accepted bytes.
func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)

	return n, err
}
