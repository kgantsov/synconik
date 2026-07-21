package storage

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/kgantsov/synconik/internal/entity"
	"github.com/rs/zerolog/log"
)

type GCSStorage struct {
	httpClient *http.Client
}

func NewGCSStorage(httpClient *http.Client) *GCSStorage {
	return &GCSStorage{
		httpClient: httpClient,
	}
}

// startUpload initiates a GCS resumable upload session against the Iconik-signed
// URL. The URL is signed for exactly this request: a POST carrying the
// "x-goog-resumable: start" extension header and no Content-Type. Adding a
// Content-Type or dropping the header changes the string GCS re-signs and yields
// a 403 SignatureDoesNotMatch. It returns the resumable session's upload ID.
func (s *GCSStorage) startUpload(uploadURL string) (string, error) {
	req, err := http.NewRequest("POST", uploadURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("x-goog-resumable", "start")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, bodyBytes)
	}

	return resp.Header.Get("X-GUploader-UploadID"), nil
}

func (s *GCSStorage) Upload(filePath string, file *entity.UploadFile) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open file: %v", err)
	}
	defer f.Close()

	fileInfo, err := f.Stat()
	if err != nil {
		return fmt.Errorf("unable to get file info: %v", err)
	}

	log.Info().Str("service", "gcs_storage").Msgf("Starting resumable upload to %s", file.UploadURL)

	uploadID, err := s.startUpload(file.UploadURL)
	if err != nil {
		return err
	}

	// The data upload is authorized by the resumable session's upload_id, so this
	// PUT is not signature-checked and can carry its own Content-Type/Length.
	fullURL := file.UploadURL + "&upload_id=" + uploadID

	req, err := http.NewRequest("PUT", fullURL, f)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = fileInfo.Size()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, bodyBytes)
	}

	return nil
}
