package storage

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Download streams the contents of a signed URL to destPath, creating any
// missing parent directories. Signed download URLs from GCS/S3/B2 are all plain
// GETs, so this is storage-method agnostic.
func Download(httpClient *http.Client, url, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("unable to create destination directory: %w", err)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, bodyBytes)
	}

	// Write to a temp file first, then rename, so a partial download never leaves
	// a truncated file at the destination path.
	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".synconik-download-*")
	if err != nil {
		return fmt.Errorf("unable to create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("error writing file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("error closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, destPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("error moving downloaded file into place: %w", err)
	}

	return nil
}
