package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestCreateAsset(t *testing.T) {
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
		assert.Equal(t, "/API/assets/v1/assets/", r.URL.Path)
		assert.Equal(t, "True", r.URL.Query().Get("assign_to_collection"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", r.Header.Get("App-ID"))
		assert.Equal(t, "abcdef0123456789abcdef0123456789", r.Header.Get("Auth-Token"))

		var asset Asset
		err := json.NewDecoder(r.Body).Decode(&asset)
		assert.NoError(t, err)
		assert.Equal(t, "Test Asset", asset.Title)
		assert.Equal(t, "ASSET", asset.Type)
		assert.Equal(t, "ACTIVE", asset.Status)
		assert.Equal(t, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", asset.CollectionID)

		asset.ID = "6ba7b811-9dad-11d1-80b4-00c04fd430c8"

		json.NewEncoder(w).Encode(asset)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, cfg.Iconik.AppID, cfg.Iconik.Token)

	asset, err := client.CreateAsset(context.Background(), &Asset{
		Title:        "Test Asset",
		Status:       "ACTIVE",
		Type:         "ASSET",
		CollectionID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
	})
	assert.NoError(t, err)
	assert.NotNil(t, asset)

	assert.Equal(t, "6ba7b811-9dad-11d1-80b4-00c04fd430c8", asset.ID)
	assert.Equal(t, "Test Asset", asset.Title)
	assert.Equal(t, "ASSET", asset.Type)
	assert.Equal(t, "ACTIVE", asset.Status)
	assert.Equal(t, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", asset.CollectionID)
}
