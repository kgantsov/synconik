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

func TestCreateAssetFormat(t *testing.T) {
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
		assert.Equal(t, fmt.Sprintf("/API/files/v1/assets/%s/formats/", "6ba7b810-9dad-11d1-80b4-00c04fd430c8"), r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", r.Header.Get("App-ID"))
		assert.Equal(t, "abcdef0123456789abcdef0123456789", r.Header.Get("Auth-Token"))

		var format Format
		err := json.NewDecoder(r.Body).Decode(&format)
		assert.NoError(t, err)
		assert.Equal(t, "Test Format", format.Name)
		assert.Equal(t, true, format.IsOnline)
		assert.Equal(t, "ACTIVE", format.Status)

		format.ID = "6ba7b811-9dad-11d1-80b4-00c04fd430c8"

		json.NewEncoder(w).Encode(format)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, cfg.Iconik.AppID, cfg.Iconik.Token)

	format, err := client.CreateAssetFormat(context.Background(), "6ba7b810-9dad-11d1-80b4-00c04fd430c8", &Format{
		Name:     "Test Format",
		IsOnline: true,
		Status:   "ACTIVE",
	})
	assert.NoError(t, err)
	assert.NotNil(t, format)

	assert.Equal(t, "6ba7b811-9dad-11d1-80b4-00c04fd430c8", format.ID)
	assert.Equal(t, "Test Format", format.Name)
	assert.Equal(t, true, format.IsOnline)
	assert.Equal(t, "ACTIVE", format.Status)
}
