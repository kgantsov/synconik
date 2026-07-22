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

func TestReconcileDeletions(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir() + "/"

	// A file that still exists on disk — must NOT be touched.
	presentPath := "present.jpg"
	err := os.WriteFile(filepath.Join(dir, presentPath), []byte("test"), 0644)
	assert.NoError(t, err)
	err = store.SaveFile(presentPath, &entity.File{
		Name:           presentPath,
		Type:           "FILE",
		AssetID:        "ASSET-PRESENT",
		FileSetID:      "CLOUD-FS-PRESENT",
		LocalStorageID: "LOCAL-STORAGE",
		LocalFileSetID: "LOCAL-FS-PRESENT",
		LocalFileID:    "LOCAL-FILE-PRESENT",
	})
	assert.NoError(t, err)

	// A file that was deleted from disk — its local file_set must be removed.
	gonePath := "gone.jpg"
	err = store.SaveFile(gonePath, &entity.File{
		Name:           gonePath,
		Type:           "FILE",
		AssetID:        "ASSET-GONE",
		FileSetID:      "CLOUD-FS-GONE",
		LocalStorageID: "LOCAL-STORAGE",
		LocalFileSetID: "LOCAL-FS-GONE",
		LocalFileID:    "LOCAL-FILE-GONE",
	})
	assert.NoError(t, err)

	cfg := &config.Config{Scanner: config.ScannerConfig{Dir: dir, Interval: 10}}

	mockClient := client.NewMockClient()
	mockClient.On("DeleteFileSet", mock.Anything, "ASSET-GONE", "LOCAL-FS-GONE").Return(nil)

	uc := NewReconcileUseCase(cfg, mockClient, store)
	uc.ReconcileDeletions()

	// Only the deleted file's local file_set is removed.
	mockClient.AssertCalled(t, "DeleteFileSet", mock.Anything, "ASSET-GONE", "LOCAL-FS-GONE")
	mockClient.AssertNotCalled(t, "DeleteFileSet", mock.Anything, "ASSET-PRESENT", "LOCAL-FS-PRESENT")

	// The deleted file's store record is dropped entirely so the next scan treats
	// it as a cache miss and can re-discover the still-present cloud copy.
	_, err = store.GetFile(gonePath)
	assert.Error(t, err)

	// The present file is untouched.
	present, err := store.GetFile(presentPath)
	assert.NoError(t, err)
	assert.Equal(t, "LOCAL-FS-PRESENT", present.LocalFileSetID)
}
