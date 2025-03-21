package usecase

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/iconik/client"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupTestDB(t *testing.T) (*store.BadgerStore, string, func()) {
	// Create a temporary directory for the test database
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	assert.NoError(t, err)

	// Create a new BadgerStore instance
	store, err := store.NewBadgerStore(tmpDir)
	assert.NoError(t, err)

	// Return cleanup function
	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, tmpDir, cleanup
}

func TestCreateCollectionIfNotExists(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir()

	imagesDir := filepath.Join(dir, "images")
	err := os.MkdirAll(imagesDir, 0755)
	assert.NoError(t, err)

	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Dir:      dir,
			Interval: 10,
		},
	}
	client := client.NewMockClient()

	dirInfo, err := os.Stat(dir)
	assert.NoError(t, err)

	client.On("CreateCollection", mock.Anything, &icnk_client.Collection{
		Title: dirInfo.Name(),
	}).Return(&icnk_client.Collection{
		ID:    "47265105-BE2B-4C3F-8997-66BAB2893D0D",
		Title: dirInfo.Name(),
	}, nil)

	uc := NewCollectionUseCase(cfg, client, store)

	err = uc.CreateCollectionIfNotExists(dir, dirInfo)
	assert.NoError(t, err)

	collection, err := store.GetFile(dir)
	assert.NoError(t, err)
	assert.Equal(t, collection.ID, "47265105-BE2B-4C3F-8997-66BAB2893D0D")

	client.On("CreateCollection", mock.Anything, &icnk_client.Collection{
		Title:    "images",
		ParentID: "47265105-BE2B-4C3F-8997-66BAB2893D0D",
	}).Return(&icnk_client.Collection{
		ID:    "FA571257-2A44-4719-AD17-7D5AD79FA23E",
		Title: "images",
	}, nil)

	imagesDirInfo, err := os.Stat(imagesDir)
	assert.NoError(t, err)

	err = uc.CreateCollectionIfNotExists(imagesDir, imagesDirInfo)
	assert.NoError(t, err)

	collection, err = store.GetFile(imagesDir)
	assert.NoError(t, err)
	assert.Equal(t, collection.ID, "FA571257-2A44-4719-AD17-7D5AD79FA23E")

}
