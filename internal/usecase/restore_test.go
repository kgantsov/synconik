package usecase

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/entity"
	"github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRestoreRequested(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir() + "/"

	// A previously-deleted local file whose cloud copy is still recorded.
	relPath := "sub/photo.jpg"
	err := store.SaveFile(relPath, &entity.File{
		DirectoryPath: "sub/",
		Name:          "photo.jpg",
		Type:          "FILE",
		AssetID:       "ASSET-1",
		FileSetID:     "CLOUD-FS-1",
		ID:            "CLOUD-FILE-1",
	})
	assert.NoError(t, err)

	// The transfer's original_url serves the source bytes.
	originalBytes := []byte("the original photo bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(originalBytes)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: dir, Interval: 10},
		Iconik:  config.Iconik{LocalStorageID: "LOCAL-STORAGE"},
	}

	mockClient := client.NewMockClient()

	// Iconik has one pending transfer of a file set TO the local storage. Pending
	// transfers come back without original_url, so the signed URL is generated.
	mockClient.On("GetStorageTransfersTo", mock.Anything, "LOCAL-STORAGE").Return([]client.Transfer{
		{
			ID:                       "TRANSFER-1",
			AssetID:                  "ASSET-1",
			FormatID:                 "FMT-1",
			OriginalStorageID:        "CLOUD-STORAGE",
			LocalStorageID:           "LOCAL-STORAGE",
			DestinationDirectoryPath: "sub/",
			DestinationFilename:      "photo.jpg",
			DestinationFileSetName:   "photo.jpg",
			AddFileSet:               true,
		},
	}, nil)

	// The signed URL of the source (cloud) copy is resolved from the asset's files.
	mockClient.On("GetAssetFiles", mock.Anything, "ASSET-1", true).Return([]client.File{
		{ID: "CLOUD-FILE-1", StorageID: "CLOUD-STORAGE", Status: "CLOSED", URL: srv.URL},
	}, nil)

	// The local file_set + file are created and closed after download.
	mockClient.On("CreateFileSet", mock.Anything, "ASSET-1", &client.FileSet{
		FormatID:     "FMT-1",
		StorageID:    "LOCAL-STORAGE",
		BaseDir:      "sub/",
		Name:         "photo.jpg",
		ComponentIds: []string{},
	}).Return(&client.FileSet{ID: "LOCAL-FS-1"}, nil)

	mockClient.On("CreateFile", mock.Anything, "ASSET-1", mock.Anything).Return(&client.File{ID: "LOCAL-FILE-1"}, nil)
	mockClient.On("CloseFile", mock.Anything, "ASSET-1", "LOCAL-FILE-1").Return(nil)

	// The handled transfer is acknowledged as completed.
	mockClient.On("AckStorageTransferTo", mock.Anything, "LOCAL-STORAGE", "TRANSFER-1", true).Return(nil)

	uc := NewRestoreUseCase(cfg, mockClient, store)
	uc.RestoreRequested()

	// The original was written to the correct local path.
	got, err := os.ReadFile(filepath.Join(dir, relPath))
	assert.NoError(t, err)
	assert.Equal(t, originalBytes, got)

	// The store entry regained its local IDs, keeping the cloud IDs.
	entry, err := store.GetFile(relPath)
	assert.NoError(t, err)
	assert.Equal(t, "ASSET-1", entry.AssetID)
	assert.Equal(t, "CLOUD-FS-1", entry.FileSetID)
	assert.Equal(t, "LOCAL-STORAGE", entry.LocalStorageID)
	assert.Equal(t, "LOCAL-FS-1", entry.LocalFileSetID)
	assert.Equal(t, "LOCAL-FILE-1", entry.LocalFileID)

	mockClient.AssertExpectations(t)
}
