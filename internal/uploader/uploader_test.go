package uploader

import (
	"os"
	"testing"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUploader(t *testing.T) {
	cfg := &config.Config{}
	cfg.ConfigureLogger()
	cfg.Scanner.Dir = "test_dir_for_scan"
	cfg.Scanner.Interval = 10
	cfg.Uploader.Workers = 5
	cfg.Iconik.StorageID = "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F"
	tmpDbDir, err := os.MkdirTemp("", "scanner-test-db-*")
	assert.NoError(t, err)

	store, err := store.NewBadgerStore(tmpDbDir)
	assert.NoError(t, err)

	mockClient := client.NewMockClient()

	mockClient.On(
		"GetStorage", mock.Anything, "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F",
	).Return(&client.Storage{ID: "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F"}, nil)

	uploadQueue := make(chan Job, 100)

	uploader := NewUploader(cfg, store, mockClient, uploadQueue)
	assert.NotNil(t, uploader)

	err = uploader.Start()
	assert.NoError(t, err)

	assert.Equal(t, len(uploader.Workers), 5)

	uploader.Stop()
}
