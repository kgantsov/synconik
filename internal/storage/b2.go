package storage

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/kgantsov/synconik/internal/entity"
)

type B2Storage struct {
	httpClient *http.Client
}

func NewB2Storage(httpClient *http.Client) *B2Storage {
	return &B2Storage{
		httpClient: httpClient,
	}
}

func (s *B2Storage) Upload(filePath string, file *entity.UploadFile) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	sha1Hash, err := ComputeSHA1(filePath)
	if err != nil {
		return fmt.Errorf("failed to compute SHA-1: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", file.UploadURL, f)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	// Ensure Content-Length is set (to avoid chunked transfer encoding)
	req.ContentLength = file.Size

	fullFileName := file.DirectoryPath + "/" + file.Name

	if file.DirectoryPath == "" {
		fullFileName = file.Name
	}

	// Set headers
	req.Header.Set("Authorization", file.UploadCredentials["authorizationToken"])
	req.Header.Set("X-Bz-File-Name", fullFileName)
	req.Header.Set("Content-Type", "b2/x-auto")
	req.Header.Set("X-Bz-Content-Sha1", sha1Hash)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", file.Size))

	// Perform the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // Read error response for debugging
		return fmt.Errorf("upload failed: %s - %s", resp.Status, string(body))
	}

	return nil
}
