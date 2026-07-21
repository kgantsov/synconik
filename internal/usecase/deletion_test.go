package usecase

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/entity"
	"github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessDeletions_DeletesLocalFile(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir() + "/"

	// A synced file present on disk and recorded in the store with local IDs.
	relPath := "sub/photo.jpg"
	assert.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, relPath), []byte("bytes"), 0644))
	assert.NoError(t, store.SaveFile(relPath, &entity.File{
		DirectoryPath:  "sub/",
		Name:           "photo.jpg",
		Type:           "FILE",
		AssetID:        "ASSET-1",
		FileSetID:      "CLOUD-FS-1",
		ID:             "CLOUD-FILE-1",
		LocalStorageID: "LOCAL-STORAGE",
		LocalFileSetID: "LOCAL-FS-1",
		LocalFileID:    "LOCAL-FILE-1",
	}))

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: dir, Interval: 10},
		Iconik:  config.Iconik{LocalStorageID: "LOCAL-STORAGE"},
	}

	mockClient := client.NewMockClient()

	// Local storage has the delete flag enabled.
	mockClient.On("GetStorage", mock.Anything, "LOCAL-STORAGE").Return(&client.Storage{
		ID:       "LOCAL-STORAGE",
		Method:   "FILE",
		Settings: map[string]interface{}{"delete": true},
	}, nil)

	// One pending deletion record for the file. Its filename is Iconik's storage
	// name (file id appended), which differs from the on-disk "photo.jpg" — the
	// on-disk path must be resolved from the store via file_id, not this name.
	mockClient.On("GetStorageDeletions", mock.Anything, "LOCAL-STORAGE").Return([]client.FileDeletion{
		{
			ID:            "DEL-1",
			AssetID:       "ASSET-1",
			DirectoryPath: "sub/",
			Filename:      "photo_LOCAL-FILE-1.jpg",
			FileID:        "LOCAL-FILE-1",
			StorageID:     "LOCAL-STORAGE",
		},
	}, nil)
	mockClient.On("DeleteStorageDeletion", mock.Anything, "LOCAL-STORAGE", "DEL-1").Return(nil)

	uc := NewDeletionUseCase(cfg, mockClient, store)
	uc.ProcessDeletions()

	// File removed from disk.
	_, err := os.Stat(filepath.Join(dir, relPath))
	assert.True(t, os.IsNotExist(err))

	// Local IDs cleared, cloud IDs kept.
	entry, err := store.GetFile(relPath)
	assert.NoError(t, err)
	assert.Equal(t, "ASSET-1", entry.AssetID)
	assert.Equal(t, "CLOUD-FS-1", entry.FileSetID)
	assert.Empty(t, entry.LocalFileSetID)
	assert.Empty(t, entry.LocalFileID)
	assert.Empty(t, entry.LocalStorageID)

	mockClient.AssertExpectations(t)
}

func TestProcessDeletions_KeepSource(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir() + "/"
	relPath := "photo.jpg"
	assert.NoError(t, os.WriteFile(filepath.Join(dir, relPath), []byte("bytes"), 0644))
	assert.NoError(t, store.SaveFile(relPath, &entity.File{
		Name:           "photo.jpg",
		Type:           "FILE",
		AssetID:        "ASSET-1",
		LocalStorageID: "LOCAL-STORAGE",
		LocalFileSetID: "LOCAL-FS-1",
		LocalFileID:    "LOCAL-FILE-1",
	}))

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: dir, Interval: 10},
		Iconik:  config.Iconik{LocalStorageID: "LOCAL-STORAGE"},
	}

	mockClient := client.NewMockClient()
	mockClient.On("GetStorage", mock.Anything, "LOCAL-STORAGE").Return(&client.Storage{
		ID:       "LOCAL-STORAGE",
		Method:   "FILE",
		Settings: map[string]interface{}{"delete": true},
	}, nil)

	// keep_source: the on-disk file must be left in place, record still acked.
	// The record's filename is Iconik's storage name, deliberately different from
	// the on-disk name to prove correlation is by file_id, not filename.
	mockClient.On("GetStorageDeletions", mock.Anything, "LOCAL-STORAGE").Return([]client.FileDeletion{
		{
			ID:            "DEL-1",
			FileID:        "LOCAL-FILE-1",
			DirectoryPath: "",
			Filename:      "photo_LOCAL-FILE-1.jpg",
			StorageID:     "LOCAL-STORAGE",
			KeepSource:    true,
		},
	}, nil)
	mockClient.On("DeleteStorageDeletion", mock.Anything, "LOCAL-STORAGE", "DEL-1").Return(nil)

	uc := NewDeletionUseCase(cfg, mockClient, store)
	uc.ProcessDeletions()

	// File still on disk.
	_, err := os.Stat(filepath.Join(dir, relPath))
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestProcessDeletions_DeleteFlagOff(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir() + "/"
	relPath := "photo.jpg"
	assert.NoError(t, os.WriteFile(filepath.Join(dir, relPath), []byte("bytes"), 0644))

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: dir, Interval: 10},
		Iconik:  config.Iconik{LocalStorageID: "LOCAL-STORAGE"},
	}

	mockClient := client.NewMockClient()
	// Delete flag off: the deletion queue is not polled, nothing is deleted.
	mockClient.On("GetStorage", mock.Anything, "LOCAL-STORAGE").Return(&client.Storage{
		ID:       "LOCAL-STORAGE",
		Method:   "FILE",
		Settings: map[string]interface{}{"delete": false},
	}, nil)

	uc := NewDeletionUseCase(cfg, mockClient, store)
	uc.ProcessDeletions()

	_, err := os.Stat(filepath.Join(dir, relPath))
	assert.NoError(t, err) // still there

	mockClient.AssertNotCalled(t, "GetStorageDeletions", mock.Anything, mock.Anything)
	mockClient.AssertExpectations(t)
}
