package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/entity"
	"github.com/kgantsov/synconik/internal/storage"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestGetStorage(t *testing.T) {
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
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, fmt.Sprintf("/API/files/v1/storages/%s/", "6ba7b811-9dad-11d1-80b4-00c04fd430c8"), r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", r.Header.Get("App-ID"))
		assert.Equal(t, "abcdef0123456789abcdef0123456789", r.Header.Get("Auth-Token"))

		storage := Storage{
			ID:      "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
			Name:    "Test Storage",
			Method:  "s3",
			Purpose: "archive",
			Status:  "active",
			Settings: map[string]interface{}{
				"bucket": "test-bucket",
				"region": "us-east-1",
			},
		}

		json.NewEncoder(w).Encode(storage)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, cfg.Iconik.AppID, cfg.Iconik.Token)

	storage, err := client.GetStorage(context.Background(), "6ba7b811-9dad-11d1-80b4-00c04fd430c8")
	assert.NoError(t, err)
	assert.NotNil(t, storage)

	assert.Equal(t, "6ba7b811-9dad-11d1-80b4-00c04fd430c8", storage.ID)
	assert.Equal(t, "Test Storage", storage.Name)
	assert.Equal(t, "s3", storage.Method)
	assert.Equal(t, "archive", storage.Purpose)
	assert.Equal(t, "active", storage.Status)
	assert.Equal(t, map[string]interface{}{
		"bucket": "test-bucket",
		"region": "us-east-1",
	}, storage.Settings)
}

func TestStorageSettingAccessors(t *testing.T) {
	// Mirrors the shape Iconik returns: JSON numbers decode as float64 and
	// scan_ignore as []interface{}.
	s := &Storage{Settings: map[string]interface{}{
		"read":                  true,
		"write":                 true,
		"delete":                false,
		"scan":                  true,
		"scan_interval_seconds": float64(5),
		"scan_ignore":           []interface{}{"*.arw", "media cache", 42, ""},
	}}

	assert.True(t, s.ReadEnabled())
	assert.True(t, s.WriteEnabled())
	assert.False(t, s.DeleteEnabled())
	assert.True(t, s.ScanEnabled())
	assert.Equal(t, 5, s.ScanIntervalSeconds())
	assert.Equal(t, []string{"*.arw", "media cache"}, s.ScanIgnore())
}

func TestStorageSettingAccessors_Defaults(t *testing.T) {
	s := &Storage{}

	assert.False(t, s.ReadEnabled())
	assert.False(t, s.WriteEnabled())
	assert.False(t, s.DeleteEnabled())
	assert.False(t, s.ScanEnabled())
	assert.Equal(t, 0, s.ScanIntervalSeconds())
	assert.Nil(t, s.ScanIgnore())
}

func TestUpload(t *testing.T) {
	cfg := &config.Config{
		Iconik: config.Iconik{
			URL:   "https://app.iconik.io",
			AppID: "123e4567-e89b-12d3-a456-426614174000",
			Token: "abcdef0123456789abcdef0123456789",
		},
	}

	client := NewClient(&http.Client{}, cfg.Iconik.URL, cfg.Iconik.AppID, cfg.Iconik.Token)

	mockStorage := storage.NewMockStorage()
	mockStorage.On("Upload", "test.mp4", &entity.UploadFile{
		Name:              "test.mp4",
		OriginalName:      "test.mp4",
		DirectoryPath:     "/test/path",
		Size:              1024,
		Type:              "video/mp4",
		StorageID:         "storage-123",
		FileSetID:         "file-set-123",
		FormatID:          "format-123",
		UploadURL:         "https://storage.test/upload",
		UploadCredentials: map[string]string{"key": "value"},
		ID:                "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
		FileDateCreated:   "2023-01-01T00:00:00Z",
		FileDateModified:  "2023-01-01T00:00:00Z",
	}).Return(nil)

	err := client.Upload(context.Background(), mockStorage, "test.mp4", &File{
		Name:              "test.mp4",
		OriginalName:      "test.mp4",
		DirectoryPath:     "/test/path",
		Size:              1024,
		Type:              "video/mp4",
		StorageID:         "storage-123",
		FileSetID:         "file-set-123",
		FormatID:          "format-123",
		UploadURL:         "https://storage.test/upload",
		UploadCredentials: map[string]string{"key": "value"},
		ID:                "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
		FileDateCreated:   "2023-01-01T00:00:00Z",
		FileDateModified:  "2023-01-01T00:00:00Z",
	})
	assert.NoError(t, err)
}
