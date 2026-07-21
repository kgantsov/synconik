package usecase

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/entity"
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

	// The parent has no existing "images" sub-collection in Iconik, so one is created.
	client.On("SearchCollections", mock.Anything, "47265105-BE2B-4C3F-8997-66BAB2893D0D", "images").
		Return([]icnk_client.Collection{}, nil)

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

// TestCreateCollectionRootCollection verifies a top-level directory (no parent in the
// store) is nested under the configured root collection_id.
func TestCreateCollectionRootCollection(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir()
	yearDir := filepath.Join(dir, "2026")
	err := os.MkdirAll(yearDir, 0755)
	assert.NoError(t, err)

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: dir + "/", Interval: 10},
		Iconik:  config.Iconik{CollectionID: "ROOT-COLLECTION"},
	}
	client := client.NewMockClient()

	yearInfo, err := os.Stat(yearDir)
	assert.NoError(t, err)

	// The scanner maps the scan root ("") to the configured collection_id first, so a
	// top-level "2026" resolves that as its parent from the store. No existing "2026"
	// under it, so it is created.
	assert.NoError(t, store.SaveFile("", &entity.File{ID: "ROOT-COLLECTION", Type: "directory"}))
	client.On("SearchCollections", mock.Anything, "ROOT-COLLECTION", "2026").
		Return([]icnk_client.Collection{}, nil)
	client.On("CreateCollection", mock.Anything, &icnk_client.Collection{
		Title:    "2026",
		ParentID: "ROOT-COLLECTION",
	}).Return(&icnk_client.Collection{
		ID:    "YEAR-COLLECTION",
		Title: "2026",
	}, nil)

	uc := NewCollectionUseCase(cfg, client, store)

	err = uc.CreateCollectionIfNotExists("2026", yearInfo)
	assert.NoError(t, err)

	collection, err := store.GetFile("2026")
	assert.NoError(t, err)
	assert.Equal(t, collection.ID, "YEAR-COLLECTION")
	client.AssertExpectations(t)
}

// TestCreateCollectionScanRootUsesCollectionID verifies that the scan-root directory
// (relativePath == "") is mapped to the configured root collection_id instead of
// creating a new collection for it — so the existing collection acts as the root and
// no originals/originals double nesting occurs.
func TestCreateCollectionScanRootUsesCollectionID(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir()

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: dir + "/", Interval: 10},
		Iconik:  config.Iconik{CollectionID: "ROOT-COLLECTION"},
	}
	client := client.NewMockClient()

	dirInfo, err := os.Stat(dir)
	assert.NoError(t, err)

	uc := NewCollectionUseCase(cfg, client, store)

	// Scanner passes "" for the scan root itself.
	err = uc.CreateCollectionIfNotExists("", dirInfo)
	assert.NoError(t, err)

	collection, err := store.GetFile("")
	assert.NoError(t, err)
	assert.Equal(t, "ROOT-COLLECTION", collection.ID)

	// No collection is created or looked up for the scan root.
	client.AssertNotCalled(t, "CreateCollection", mock.Anything, mock.Anything)
	client.AssertNotCalled(t, "SearchCollections", mock.Anything, mock.Anything, mock.Anything)
}

// TestCreateCollectionScanRootNoCollectionID verifies that without a collection_id the
// scan-root directory still gets its own collection created (the root directory).
func TestCreateCollectionScanRootNoCollectionID(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir()

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: dir + "/", Interval: 10},
	}
	client := client.NewMockClient()

	dirInfo, err := os.Stat(dir)
	assert.NoError(t, err)

	// No parent (true root), so no SearchCollections lookup; a collection is created.
	client.On("CreateCollection", mock.Anything, &icnk_client.Collection{
		Title: dirInfo.Name(),
	}).Return(&icnk_client.Collection{
		ID:    "SCAN-ROOT-COLLECTION",
		Title: dirInfo.Name(),
	}, nil)

	uc := NewCollectionUseCase(cfg, client, store)

	err = uc.CreateCollectionIfNotExists("", dirInfo)
	assert.NoError(t, err)

	collection, err := store.GetFile("")
	assert.NoError(t, err)
	assert.Equal(t, "SCAN-ROOT-COLLECTION", collection.ID)
	client.AssertExpectations(t)
}

// TestCreateCollectionAlreadyExistsInIconik verifies that when a sub-collection with
// the same title already exists under the parent in Iconik (e.g. the local store was
// wiped), it is reused and no duplicate is created.
func TestCreateCollectionAlreadyExistsInIconik(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir()
	yearDir := filepath.Join(dir, "2026")
	err := os.MkdirAll(yearDir, 0755)
	assert.NoError(t, err)

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: dir + "/", Interval: 10},
		Iconik:  config.Iconik{CollectionID: "ROOT-COLLECTION"},
	}
	client := client.NewMockClient()

	yearInfo, err := os.Stat(yearDir)
	assert.NoError(t, err)

	// The scan root ("") maps to the configured collection_id, resolved as the parent
	// of a top-level "2026". Iconik already has a "2026" under it -> reuse it.
	assert.NoError(t, store.SaveFile("", &entity.File{ID: "ROOT-COLLECTION", Type: "directory"}))
	client.On("SearchCollections", mock.Anything, "ROOT-COLLECTION", "2026").
		Return([]icnk_client.Collection{
			{ID: "OTHER", Title: "2025"},
			{ID: "EXISTING-2026", Title: "2026"},
		}, nil)

	uc := NewCollectionUseCase(cfg, client, store)

	err = uc.CreateCollectionIfNotExists("2026", yearInfo)
	assert.NoError(t, err)

	collection, err := store.GetFile("2026")
	assert.NoError(t, err)
	assert.Equal(t, "EXISTING-2026", collection.ID)

	client.AssertNotCalled(t, "CreateCollection", mock.Anything, mock.Anything)
	client.AssertExpectations(t)
}
