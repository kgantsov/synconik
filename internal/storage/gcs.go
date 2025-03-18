package storage

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
)

type GCSStorage struct {
	httpClient *http.Client
}

func NewGCSStorage(httpClient *http.Client) *GCSStorage {
	return &GCSStorage{
		httpClient: httpClient,
	}
}

func (s *GCSStorage) startUpload(upload_url string) (string, error) {
	req, err := http.NewRequest("POST", upload_url, nil)
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
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, bodyBytes)
	}

	uploadID := resp.Header.Get("X-GUploader-UploadID")

	return uploadID, nil
}

func (s *GCSStorage) Upload(filePath string, file *icnk_client.File) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open file: %v", err)
	}
	defer f.Close()

	uploadID, err := s.startUpload(file.UploadURL)
	if err != nil {
		return err
	}

	full_url := file.UploadURL + "&upload_id=" + uploadID

	req, err := http.NewRequest("PUT", full_url, f)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("x-goog-resumable", "start")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, bodyBytes)
	}

	return nil
}
