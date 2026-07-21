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

func TestStripStoragePrefix(t *testing.T) {
	cases := []struct {
		name   string
		dir    string
		prefix string
		want   string
	}{
		{"strips matching prefix", "originals/2001", "originals", "2001"},
		{"strips matching prefix with trailing slash", "originals/2001/", "originals", "2001/"},
		{"strips matching prefix with leading slash", "/originals/2001", "originals", "2001"},
		{"exact match collapses to empty", "originals", "originals", ""},
		{"exact match with leading slash collapses to empty", "/originals", "originals", ""},
		{"leaves non-matching prefix untouched", "2001", "originals", "2001"},
		{"only strips the prefix segment once", "originals/originals/2001", "originals", "originals/2001"},
		{"does not strip a partial segment match", "originals-backup/2001", "originals", "originals-backup/2001"},
		{"empty prefix is a no-op", "originals/2001", "", "originals/2001"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, stripStoragePrefix(c.dir, c.prefix))
		})
	}
}

// TestRestoreRequestedStripsScanDirPrefix reproduces the real-world doubling bug:
// scanner.dir ends in "originals" and Iconik reports the transfer destination using
// the cloud file's directory_path ("originals/2001/"), which carries that same
// scan-root folder name. The prefix must be stripped so the original lands in
// scanner.dir/2001 and not scanner.dir/originals/2001.
func TestRestoreRequestedStripsScanDirPrefix(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	// scanner.dir's last segment ("originals") is the storage prefix Iconik prepends
	// to cloud directory_paths, so it comes back on the transfer destination.
	dir := t.TempDir() + "/originals/"
	assert.NoError(t, os.MkdirAll(dir, 0o755))

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

	mockClient.On("GetStorageTransfersTo", mock.Anything, "LOCAL-STORAGE").Return([]client.Transfer{
		{
			ID:                       "TRANSFER-1",
			AssetID:                  "ASSET-1",
			FormatID:                 "FMT-1",
			OriginalStorageID:        "CLOUD-STORAGE",
			LocalStorageID:           "LOCAL-STORAGE",
			DestinationDirectoryPath: "originals/2001/",
			DestinationFilename:      "photo.jpg",
			DestinationFileSetName:   "photo.jpg",
			AddFileSet:               true,
		},
	}, nil)

	mockClient.On("GetAssetFiles", mock.Anything, "ASSET-1", true).Return([]client.File{
		{ID: "CLOUD-FILE-1", StorageID: "CLOUD-STORAGE", Status: "CLOSED", URL: srv.URL},
	}, nil)

	// The local file_set is created with the prefix-stripped base dir ("2001/").
	mockClient.On("CreateFileSet", mock.Anything, "ASSET-1", &client.FileSet{
		FormatID:     "FMT-1",
		StorageID:    "LOCAL-STORAGE",
		BaseDir:      "2001/",
		Name:         "photo.jpg",
		ComponentIds: []string{},
	}).Return(&client.FileSet{ID: "LOCAL-FS-1"}, nil)

	mockClient.On("CreateFile", mock.Anything, "ASSET-1", mock.Anything).Return(&client.File{ID: "LOCAL-FILE-1"}, nil)
	mockClient.On("CloseFile", mock.Anything, "ASSET-1", "LOCAL-FILE-1").Return(nil)
	mockClient.On("AckStorageTransferTo", mock.Anything, "LOCAL-STORAGE", "TRANSFER-1", true).Return(nil)

	uc := NewRestoreUseCase(cfg, mockClient, store)
	uc.RestoreRequested()

	// The original landed at scanner.dir/2001/photo.jpg (prefix stripped)...
	got, err := os.ReadFile(filepath.Join(dir, "2001", "photo.jpg"))
	assert.NoError(t, err)
	assert.Equal(t, originalBytes, got)

	// ...and NOT at the doubled scanner.dir/originals/2001/photo.jpg.
	_, err = os.Stat(filepath.Join(dir, "originals", "2001", "photo.jpg"))
	assert.True(t, os.IsNotExist(err), "file must not be written to the doubled originals/originals path")

	entry, err := store.GetFile("2001/photo.jpg")
	assert.NoError(t, err)
	assert.Equal(t, "ASSET-1", entry.AssetID)
	assert.Equal(t, "LOCAL-FS-1", entry.LocalFileSetID)
	assert.Equal(t, "LOCAL-FILE-1", entry.LocalFileID)

	mockClient.AssertExpectations(t)
}
