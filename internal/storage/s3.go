package storage

import (
	"fmt"
	"net/http"
	"os"

	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
)

type S3Storage struct {
	httpClient *http.Client
}

func NewS3Storage(httpClient *http.Client) *S3Storage {
	return &S3Storage{
		httpClient: httpClient,
	}
}

func (s *S3Storage) Upload(filePath string, file *icnk_client.File) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	req, err := http.NewRequest("PUT", file.UploadURL, f)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload failed with status: %s", resp.Status)
	}

	return nil
}
