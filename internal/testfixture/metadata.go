package testfixture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

const (
	indexName       = "index.json"
	updateJSONName  = "update.json"
	updateSJSONName = "update.sjson"
)

// publishedAt is a fixed UTC timestamp so index.json and update.json are
// bit-identical across Generate calls.
func publishedAt() time.Time {
	return time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC)
}

// writeMetadata writes index.json, <version>/update.json, and
// <version>/update.sjson. All three share one Files list for the
// three-way Filename/Sha256/Size binding.
func writeMetadata(dir string, files []apiimages.UpdateFile) error {
	update := apiimages.Update{
		Format:      Format,
		Channels:    []string{ChannelStable},
		Files:       files,
		Origin:      Origin,
		PublishedAt: publishedAt(),
		Severity:    apiimages.UpdateSeverityHigh,
		Version:     Version,
	}

	updateJSON, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("marshal update.json: %w", err)
	}

	index := apiimages.Index{
		Format: Format,
		Updates: []apiimages.UpdateFull{{
			Update: update,
			URL:    "/" + Version,
		}},
	}
	indexJSON, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("marshal index.json: %w", err)
	}

	err = os.WriteFile(filepath.Join(dir, indexName), indexJSON, filePerm)
	if err != nil {
		return fmt.Errorf("write index.json: %w", err)
	}

	verDir := versionDir(dir)
	err = os.MkdirAll(verDir, dirPerm)
	if err != nil {
		return fmt.Errorf("create version directory: %w", err)
	}

	err = os.WriteFile(filepath.Join(verDir, updateJSONName), updateJSON, filePerm)
	if err != nil {
		return fmt.Errorf("write update.json: %w", err)
	}

	sjson, err := signedSJSON(updateJSON)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(verDir, updateSJSONName), sjson, filePerm)
	if err != nil {
		return fmt.Errorf("write update.sjson: %w", err)
	}

	return nil
}
