package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestCreateFile(t *testing.T) {
	cfg := &config.Config{
		Iconik: config.Iconik{
			URL:   "https://app.iconik.io",
			AppID: "123e4567-e89b-12d3-a456-426614174000",
			Token: "abcdef0123456789abcdef0123456789",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Str("service", "iconik_client").Msg("Request received")
		// Verify request headers
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, fmt.Sprintf("/API/files/v1/assets/%s/files/", "6ba7b810-9dad-11d1-80b4-00c04fd430c8"), r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", r.Header.Get("App-ID"))
		assert.Equal(t, "abcdef0123456789abcdef0123456789", r.Header.Get("Auth-Token"))

		var file File
		err := json.NewDecoder(r.Body).Decode(&file)
		assert.NoError(t, err)
		assert.Equal(t, "test.mp4", file.Name)
		assert.Equal(t, "test.mp4", file.OriginalName)
		assert.Equal(t, "/test/path", file.DirectoryPath)
		assert.Equal(t, int64(1024), file.Size)
		assert.Equal(t, "video/mp4", file.Type)
		assert.Equal(t, "storage-123", file.StorageID)
		assert.Equal(t, "file-set-123", file.FileSetID)
		assert.Equal(t, "format-123", file.FormatID)

		file.ID = "6ba7b811-9dad-11d1-80b4-00c04fd430c8"
		file.UploadURL = "https://storage.test/upload"
		file.UploadCredentials = map[string]string{
			"key": "value",
		}

		json.NewEncoder(w).Encode(file)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, cfg.Iconik.AppID, cfg.Iconik.Token)

	file, err := client.CreateFile(context.Background(), "6ba7b810-9dad-11d1-80b4-00c04fd430c8", &File{
		Name:          "test.mp4",
		OriginalName:  "test.mp4",
		DirectoryPath: "/test/path",
		Size:          1024,
		Type:          "video/mp4",
		StorageID:     "storage-123",
		FileSetID:     "file-set-123",
		FormatID:      "format-123",
	})
	assert.NoError(t, err)
	assert.NotNil(t, file)

	assert.Equal(t, "6ba7b811-9dad-11d1-80b4-00c04fd430c8", file.ID)
	assert.Equal(t, "test.mp4", file.Name)
	assert.Equal(t, "test.mp4", file.OriginalName)
	assert.Equal(t, "/test/path", file.DirectoryPath)
	assert.Equal(t, int64(1024), file.Size)
	assert.Equal(t, "video/mp4", file.Type)
	assert.Equal(t, "storage-123", file.StorageID)
	assert.Equal(t, "file-set-123", file.FileSetID)
	assert.Equal(t, "format-123", file.FormatID)
	assert.Equal(t, "https://storage.test/upload", file.UploadURL)
	assert.Equal(t, map[string]string{"key": "value"}, file.UploadCredentials)
}

func TestTriggerTranscoding(t *testing.T) {
	cfg := &config.Config{
		Iconik: config.Iconik{
			URL:   "https://app.iconik.io",
			AppID: "123e4567-e89b-12d3-a456-426614174000",
			Token: "abcdef0123456789abcdef0123456789",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Str("service", "iconik_client").Msg("Request received")
		// Verify request headers
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, fmt.Sprintf("/API/files/v1/assets/%s/files/%s/keyframes/", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "6ba7b811-9dad-11d1-80b4-00c04fd430c8"), r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", r.Header.Get("App-ID"))
		assert.Equal(t, "abcdef0123456789abcdef0123456789", r.Header.Get("Auth-Token"))

		var body struct {
			UseStorageTranscodeIgnorePattern bool `json:"use_storage_transcode_ignore_pattern"`
			Priority                         int  `json:"priority"`
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.True(t, body.UseStorageTranscodeIgnorePattern)
		assert.Equal(t, 5, body.Priority)

		json.NewEncoder(w).Encode(map[string]string{
			"job_id": "49b8b43e-074e-11f0-b783-8ab8b108bac0",
		})

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, cfg.Iconik.AppID, cfg.Iconik.Token)

	jobID, err := client.TriggerTranscoding(
		context.Background(),
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"6ba7b811-9dad-11d1-80b4-00c04fd430c8",
	)
	assert.NoError(t, err)
	assert.Equal(t, "49b8b43e-074e-11f0-b783-8ab8b108bac0", jobID)
}

func TestCloseFile(t *testing.T) {
	cfg := &config.Config{
		Iconik: config.Iconik{
			URL:   "https://app.iconik.io",
			AppID: "123e4567-e89b-12d3-a456-426614174000",
			Token: "abcdef0123456789abcdef0123456789",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Str("service", "iconik_client").Msg("Request received")

		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, fmt.Sprintf("/API/files/v1/assets/%s/files/%s/", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "6ba7b811-9dad-11d1-80b4-00c04fd430c8"), r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", r.Header.Get("App-ID"))
		assert.Equal(t, "abcdef0123456789abcdef0123456789", r.Header.Get("Auth-Token"))

		var body struct {
			Status            string `json:"status"`
			ProgressProcessed int    `json:"progress_processed"`
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "CLOSED", body.Status)
		assert.Equal(t, 100, body.ProgressProcessed)

		err = json.NewEncoder(w).Encode(body)
		assert.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, cfg.Iconik.AppID, cfg.Iconik.Token)

	err := client.CloseFile(context.Background(), "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "6ba7b811-9dad-11d1-80b4-00c04fd430c8")
	assert.NoError(t, err)
}
