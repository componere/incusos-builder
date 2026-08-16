package update

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/url"
	"strings"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	updateJSONName  = "update.json"
	updateSJSONName = "update.sjson"
	stepMetadata    = "metadata"
	signedType      = "multipart/signed"
	// metadataCap is the per-document read cap for update.json and
	// update.sjson. Spike 1.C measured the live largest document at 14_268
	// bytes (update.sjson); 1 MiB leaves ~70× headroom (~4_800 file entries)
	// vs the plan's 8 MiB unmeasured placeholder.
	metadataCap int64 = 1 << 20
)

// ReleaseMetadata fetches update.json and update.sjson from /<version>/,
// validates them structurally, and returns the HTTP bodies verbatim.
func (s *HTTPSSource) ReleaseMetadata(
	ctx context.Context,
	version string,
	selected []apiimages.UpdateFile,
) (build.ReleaseMetadata, error) {
	if err := ValidateVersion(version); err != nil {
		return build.ReleaseMetadata{}, err
	}
	s.reporter.Step(stepMetadata)
	defer s.reporter.Done(stepMetadata)
	return loadReleaseMetadata(ctx, version, selected, func(ctx context.Context, name string) ([]byte, error) {
		rawURL, err := url.JoinPath(s.base, version, name)
		if err != nil {
			return nil, fmt.Errorf("%w: metadata URL: %w", ErrFetch, err)
		}
		body, err := s.getCapped(ctx, rawURL, s.metaLimit)
		if err != nil {
			return nil, err
		}
		s.reporter.Progress(int64(len(body)), int64(len(body)))
		return body, nil
	})
}

// ReleaseMetadata reads update.json and update.sjson from
// <dir>/<version>/, validates them structurally, and returns the file
// bytes verbatim.
func (s *LocalSource) ReleaseMetadata(
	ctx context.Context,
	version string,
	selected []apiimages.UpdateFile,
) (build.ReleaseMetadata, error) {
	if err := ValidateVersion(version); err != nil {
		return build.ReleaseMetadata{}, err
	}
	s.reporter.Step(stepMetadata)
	defer s.reporter.Done(stepMetadata)
	return loadReleaseMetadata(ctx, version, selected, func(_ context.Context, name string) ([]byte, error) {
		body, err := s.readCapped(version, name, s.metaLimit)
		if err != nil {
			return nil, err
		}
		s.reporter.Progress(int64(len(body)), int64(len(body)))
		return body, nil
	})
}

// loadReleaseMetadata fetches both documents, structurally validates them,
// and returns the original bytes.
func loadReleaseMetadata(
	ctx context.Context,
	version string,
	selected []apiimages.UpdateFile,
	get func(context.Context, string) ([]byte, error),
) (build.ReleaseMetadata, error) {
	jsonBytes, err := get(ctx, updateJSONName)
	if err != nil {
		return build.ReleaseMetadata{}, err
	}
	sjsonBytes, err := get(ctx, updateSJSONName)
	if err != nil {
		return build.ReleaseMetadata{}, err
	}
	if err := validateUpdateJSON(jsonBytes, version); err != nil {
		return build.ReleaseMetadata{}, err
	}
	if err := validateUpdateSJSON(sjsonBytes, version, selected); err != nil {
		return build.ReleaseMetadata{}, err
	}
	return build.ReleaseMetadata{UpdateJSON: jsonBytes, UpdateSJSON: sjsonBytes}, nil
}

// validateUpdateJSON decodes data as apiimages.Update with Version==version.
func validateUpdateJSON(data []byte, version string) error {
	var doc apiimages.Update
	if err := decodeJSON(data, &doc, updateJSONName); err != nil {
		return err
	}
	if doc.Version != version {
		return fmt.Errorf("%w: %s version %q != %q; %s", ErrFetch, updateJSONName, doc.Version, version, tamperSuffix)
	}
	return nil
}

// validateUpdateSJSON requires a multipart/signed S/MIME message whose
// clear-text payload decodes as apiimages.Update with Version==version
// and Files covering every selected Filename+Sha256 pair.
func validateUpdateSJSON(data []byte, version string, selected []apiimages.UpdateFile) error {
	payload, err := signedCleartext(data)
	if err != nil {
		return err
	}
	var doc apiimages.Update
	if err := decodeJSON(payload, &doc, updateSJSONName); err != nil {
		return err
	}
	if doc.Version != version {
		return fmt.Errorf("%w: %s version %q != %q; %s", ErrFetch, updateSJSONName, doc.Version, version, tamperSuffix)
	}
	return bindSelected(doc.Files, selected)
}

// bindSelected requires every selected file to appear in files with an
// equal Filename and Sha256 (ARCHITECTURE §6 three-way binding).
func bindSelected(files, selected []apiimages.UpdateFile) error {
	type key struct {
		name   string
		digest string
	}
	have := make(map[key]struct{}, len(files))
	for _, f := range files {
		have[key{name: f.Filename, digest: f.Sha256}] = struct{}{}
	}
	for _, want := range selected {
		if _, ok := have[key{name: want.Filename, digest: want.Sha256}]; !ok {
			return fmt.Errorf(
				"%w: %s missing selected file %q sha256 %q; %s",
				ErrFetch,
				updateSJSONName,
				want.Filename,
				want.Sha256,
				tamperSuffix,
			)
		}
	}
	return nil
}

// signedCleartext extracts the first multipart/signed part. Each part is
// read fully before NextPart (mime/multipart invalidates the previous body).
func signedCleartext(data []byte) ([]byte, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: parse %s: %w", ErrFetch, updateSJSONName, err)
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("%w: parse %s Content-Type: %w", ErrFetch, updateSJSONName, err)
	}
	if !strings.EqualFold(mediaType, signedType) {
		return nil, fmt.Errorf("%w: %s is not multipart/signed; %s", ErrFetch, updateSJSONName, tamperSuffix)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("%w: %s missing MIME boundary; %s", ErrFetch, updateSJSONName, tamperSuffix)
	}
	reader := multipart.NewReader(msg.Body, boundary)
	part, err := reader.NextPart()
	if err != nil {
		return nil, fmt.Errorf("%w: %s has no MIME parts; %s", ErrFetch, updateSJSONName, tamperSuffix)
	}
	payload, err := io.ReadAll(part)
	_ = part.Close()
	if err != nil {
		return nil, fmt.Errorf("%w: read %s payload: %w", ErrFetch, updateSJSONName, err)
	}
	for {
		next, nerr := reader.NextPart()
		if nerr != nil {
			break
		}
		_, _ = io.ReadAll(next)
		_ = next.Close()
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, fmt.Errorf("%w: %s payload is empty; %s", ErrFetch, updateSJSONName, tamperSuffix)
	}
	return payload, nil
}
