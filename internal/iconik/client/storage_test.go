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
