package usecase

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/entity"
	"github.com/kgantsov/synconik/internal/iconik/client"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	cloudStorageID = "240E2FF6-0215-4F10-A0A4-37366C0F710B"
	localStorageID = "8C1F2A3B-LOCAL-STORAGE-ID"
)

// makeTestFile writes a real file so os.Stat yields a usable FileInfo (Name/Size/
// ModTime). The path passed to UploadAsset is independent of where this lives — the
// upload itself is mocked, so the bytes are never read.
func makeTestFile(t *testing.T, name string) os.FileInfo {
	p := filepath.Join(t.TempDir(), name)
	assert.NoError(t, os.WriteFile(p, []byte("test"), 0644))
	info, err := os.Stat(p)
	assert.NoError(t, err)
	return info
}

func testStorages() (*icnk_client.Storage, *icnk_client.Storage) {
	return &icnk_client.Storage{
			ID: cloudStorageID, Name: "test", Method: "S3", Purpose: "FILES", Status: "ACTIVE",
		},
		&icnk_client.Storage{
			ID: localStorageID, Name: "local", Method: "FILE", Purpose: "FILES", Status: "ACTIVE",
		}
}

type uploadIDs struct {
	asset, format, cloudFileSet, cloudFile, localFileSet, localFile string
}

// mockHappyUpload registers the full new-upload handshake: the storage dedup check
// returns no match, then the cloud + local file sets/files are created. cloudDir is
// the prefixed cloud directory_path; localDir is the unprefixed local one. Returns
// the *File that CreateFile yields for the cloud copy (used to match Upload).
func mockHappyUpload(
	c *client.MockClient, cloudDir, localDir, collectionID string, info os.FileInfo, ids uploadIDs,
) *icnk_client.File {
	c.On("GetStorageFiles", mock.Anything, cloudStorageID, cloudDir).
		Return([]icnk_client.File{}, nil)

	return mockCreateHandshake(c, cloudDir, localDir, collectionID, info, ids)
}

// mockCreateHandshake registers the full new-upload handshake (asset/format/file set/
// file creation, upload, transcoding, local mirror) without the storage dedup check,
// so callers can control what GetStorageFiles returns.
func mockCreateHandshake(
	c *client.MockClient, cloudDir, localDir, collectionID string, info os.FileInfo, ids uploadIDs,
) *icnk_client.File {
	created := info.ModTime().Format(time.RFC3339)

	c.On("CreateAsset", mock.Anything, &icnk_client.Asset{
		Title:        info.Name(),
		Status:       "ACTIVE",
		Type:         "ASSET",
		CollectionID: collectionID,
	}).Return(&icnk_client.Asset{ID: ids.asset}, nil)

	c.On("CreateAssetFormat", mock.Anything, ids.asset, &icnk_client.Format{
		Name:           "ORIGINAL",
		Status:         "ACTIVE",
		Metadata:       []map[string]string{{"internet_media_type": "image/jpeg"}},
		StorageMethods: []string{"S3", "FILE"},
	}).Return(&icnk_client.Format{ID: ids.format}, nil)

	// Cloud (S3) file set + file, using the prefixed directory path.
	c.On("CreateFileSet", mock.Anything, ids.asset, &icnk_client.FileSet{
		FormatID:     ids.format,
		StorageID:    cloudStorageID,
		BaseDir:      cloudDir,
		Name:         info.Name(),
		ComponentIds: []string{},
	}).Return(&icnk_client.FileSet{ID: ids.cloudFileSet}, nil)

	cloudFile := &icnk_client.File{
		ID:            ids.cloudFile,
		Name:          info.Name(),
		OriginalName:  info.Name(),
		DirectoryPath: cloudDir,
		Size:          info.Size(),
		FormatID:      ids.format,
		FileSetID:     ids.cloudFileSet,
		UploadURL:     "https://test.com",
	}
	c.On("CreateFile", mock.Anything, ids.asset, &icnk_client.File{
		OriginalName:     info.Name(),
		DirectoryPath:    cloudDir,
		Size:             info.Size(),
		Type:             "FILE",
		StorageID:        cloudStorageID,
		FormatID:         ids.format,
		FileSetID:        ids.cloudFileSet,
		FileDateCreated:  created,
		FileDateModified: created,
	}).Return(cloudFile, nil)

	c.On("Upload", mock.Anything, mock.Anything, mock.Anything, cloudFile).Return(nil)
	c.On("CloseFile", mock.Anything, ids.asset, ids.cloudFile).Return(nil)
	c.On("TriggerTranscoding", mock.Anything, ids.asset, ids.cloudFile).
		Return("C2BC2D18-FBF5-4D89-92B3-35E586ABCCD8", nil)

	// Local ("FILE" method) file set + file, using the unprefixed directory path.
	c.On("CreateFileSet", mock.Anything, ids.asset, &icnk_client.FileSet{
		FormatID:     ids.format,
		StorageID:    localStorageID,
		BaseDir:      localDir,
		Name:         info.Name(),
		ComponentIds: []string{},
	}).Return(&icnk_client.FileSet{ID: ids.localFileSet}, nil)

	c.On("CreateFile", mock.Anything, ids.asset, &icnk_client.File{
		OriginalName:     info.Name(),
		DirectoryPath:    localDir,
		Size:             info.Size(),
		Type:             "FILE",
		StorageID:        localStorageID,
		FormatID:         ids.format,
		FileSetID:        ids.localFileSet,
		FileDateCreated:  created,
		FileDateModified: created,
	}).Return(&icnk_client.File{ID: ids.localFile}, nil)

	c.On("CloseFile", mock.Anything, ids.asset, ids.localFile).Return(nil)

	return cloudFile
}

func TestUploadAsset(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	info := makeTestFile(t, "image.jpg")

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: "/data/originals/", Interval: 10},
	}
	storage, localStorage := testStorages()
	c := client.NewMockClient()
	uc := NewAssetUseCase(cfg, c, store, storage, localStorage)

	ids := uploadIDs{
		asset:        "47265105-BE2B-4C3F-8997-66BAB2893D0D",
		format:       "EDEF4933-4CB5-4FFE-B55F-C00549AC164B",
		cloudFileSet: "05BE6FD5-9B15-4C7D-8B54-5749239A89D4",
		cloudFile:    "D025605F-CF64-4EE5-9F48-E6DD5D363473",
		localFileSet: "LOCALFS-1234",
		localFile:    "LOCALFILE-5678",
	}
	// The scan-root folder ("originals") is prepended on the cloud side only.
	mockHappyUpload(c, "originals/2026/2026-05-23", "2026/2026-05-23/", "", info, ids)

	file, err := uc.UploadAsset("2026/2026-05-23/"+info.Name(), info)
	assert.NoError(t, err)
	assert.Equal(t, ids.cloudFile, file.ID)
	assert.Equal(t, localStorageID, file.LocalStorageID)
	assert.Equal(t, ids.localFileSet, file.LocalFileSetID)
	c.AssertExpectations(t)
}

// TestUploadAssetIgnoresOpenCloudFile verifies that a lingering OPEN cloud file (a
// leftover from a previously failed upload, whose bytes never landed) is not treated
// as already-existing: the full upload handshake runs instead of recording a mapping.
func TestUploadAssetIgnoresOpenCloudFile(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	info := makeTestFile(t, "image.jpg")

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: "/data/originals/", Interval: 10},
	}
	storage, localStorage := testStorages()
	c := client.NewMockClient()
	uc := NewAssetUseCase(cfg, c, store, storage, localStorage)

	// A same-named file exists on the cloud storage but is still OPEN (upload never
	// completed), so it must be ignored and a genuine upload must proceed.
	c.On("GetStorageFiles", mock.Anything, cloudStorageID, "originals/2026/2026-05-23").
		Return([]icnk_client.File{{
			ID:           "STALE-OPEN-FILE",
			OriginalName: info.Name(),
			AssetID:      "STALE-ASSET",
			Status:       "OPEN",
		}}, nil)

	ids := uploadIDs{
		asset:        "NEW-ASSET",
		format:       "NEW-FORMAT",
		cloudFileSet: "NEW-CLOUDFS",
		cloudFile:    "NEW-CLOUDFILE",
		localFileSet: "NEW-LOCALFS",
		localFile:    "NEW-LOCALFILE",
	}
	mockCreateHandshake(c, "originals/2026/2026-05-23", "2026/2026-05-23/", "", info, ids)

	file, err := uc.UploadAsset("2026/2026-05-23/"+info.Name(), info)
	assert.NoError(t, err)
	assert.Equal(t, ids.cloudFile, file.ID)
	assert.Equal(t, ids.asset, file.AssetID)
	c.AssertExpectations(t)
}

func TestUploadIfNotExists(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	info := makeTestFile(t, "image.jpg")

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: "/data/originals/", Interval: 10},
	}
	storage, localStorage := testStorages()
	c := client.NewMockClient()
	uc := NewAssetUseCase(cfg, c, store, storage, localStorage)

	ids := uploadIDs{
		asset:        "47265105-BE2B-4C3F-8997-66BAB2893D0D",
		format:       "EDEF4933-4CB5-4FFE-B55F-C00549AC164B",
		cloudFileSet: "05BE6FD5-9B15-4C7D-8B54-5749239A89D4",
		cloudFile:    "D025605F-CF64-4EE5-9F48-E6DD5D363473",
		localFileSet: "LOCALFS-1234",
		localFile:    "LOCALFILE-5678",
	}
	mockHappyUpload(c, "originals/2026/2026-05-23", "2026/2026-05-23/", "", info, ids)

	err := uc.UploadIfNotExists("2026/2026-05-23/"+info.Name(), info)
	assert.NoError(t, err)
	c.AssertExpectations(t)
}

// TestUploadAssetRootCollection verifies a root-level file (no parent directory) is
// nested under the configured root collection and uses the bare "originals" prefix.
func TestUploadAssetRootCollection(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	info := makeTestFile(t, "image.jpg")

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: "/data/originals/", Interval: 10},
		Iconik:  config.Iconik{CollectionID: "ROOT-COLLECTION"},
	}
	storage, localStorage := testStorages()
	c := client.NewMockClient()
	uc := NewAssetUseCase(cfg, c, store, storage, localStorage)

	// The scanner processes the scan root first, mapping "" to the configured root
	// collection; a root-level file then resolves that as its parent from the store.
	assert.NoError(t, store.SaveFile("", &entity.File{ID: "ROOT-COLLECTION", Type: "directory"}))

	ids := uploadIDs{
		asset:        "ASSET-1",
		format:       "FORMAT-1",
		cloudFileSet: "CLOUDFS-1",
		cloudFile:    "CLOUDFILE-1",
		localFileSet: "LOCALFS-1",
		localFile:    "LOCALFILE-1",
	}
	mockHappyUpload(c, "originals", "", "ROOT-COLLECTION", info, ids)

	file, err := uc.UploadAsset(info.Name(), info)
	assert.NoError(t, err)
	assert.Equal(t, ids.cloudFile, file.ID)
	c.AssertExpectations(t)
}

// TestUploadAssetAlreadyExists verifies that when the file already lives on the cloud
// storage we record the mapping, register the missing local mirror, and never create
// a new asset or upload bytes.
func TestUploadAssetAlreadyExists(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	info := makeTestFile(t, "image.jpg")

	cfg := &config.Config{
		Scanner: config.ScannerConfig{Dir: "/data/originals/", Interval: 10},
	}
	storage, localStorage := testStorages()
	c := client.NewMockClient()
	uc := NewAssetUseCase(cfg, c, store, storage, localStorage)

	c.On("GetStorageFiles", mock.Anything, cloudStorageID, "originals/2026/2026-05-23").
		Return([]icnk_client.File{{
			ID:           "EXISTING-FILE",
			Name:         "_DSC1286_someid.jpg", // Iconik mangles the storage name...
			OriginalName: info.Name(),           // ...so matching is on original_name
			AssetID:      "EXISTING-ASSET",
			FormatID:     "EXISTING-FORMAT",
			FileSetID:    "EXISTING-FILESET",
			StorageID:    cloudStorageID,
			Status:       "CLOSED", // only a fully-uploaded (CLOSED) file counts as existing
		}}, nil)

	// Asset has no local ("FILE") copy yet, so a local mirror file set is created.
	c.On("GetAssetFiles", mock.Anything, "EXISTING-ASSET", false).
		Return([]icnk_client.File{{ID: "EXISTING-FILE", StorageID: cloudStorageID}}, nil)

	c.On("CreateFileSet", mock.Anything, "EXISTING-ASSET", &icnk_client.FileSet{
		FormatID:     "EXISTING-FORMAT",
		StorageID:    localStorageID,
		BaseDir:      "2026/2026-05-23/",
		Name:         info.Name(),
		ComponentIds: []string{},
	}).Return(&icnk_client.FileSet{ID: "LOCALFS-NEW"}, nil)

	c.On("CreateFile", mock.Anything, "EXISTING-ASSET", &icnk_client.File{
		OriginalName:     info.Name(),
		DirectoryPath:    "2026/2026-05-23/",
		Size:             info.Size(),
		Type:             "FILE",
		StorageID:        localStorageID,
		FormatID:         "EXISTING-FORMAT",
		FileSetID:        "LOCALFS-NEW",
		FileDateCreated:  info.ModTime().Format(time.RFC3339),
		FileDateModified: info.ModTime().Format(time.RFC3339),
	}).Return(&icnk_client.File{ID: "LOCALFILE-NEW"}, nil)

	c.On("CloseFile", mock.Anything, "EXISTING-ASSET", "LOCALFILE-NEW").Return(nil)

	file, err := uc.UploadAsset("2026/2026-05-23/"+info.Name(), info)
	assert.NoError(t, err)

	assert.Equal(t, "EXISTING-FILE", file.ID)
	assert.Equal(t, "EXISTING-ASSET", file.AssetID)
	assert.Equal(t, "EXISTING-FORMAT", file.FormatID)
	assert.Equal(t, "LOCALFS-NEW", file.LocalFileSetID)
	assert.Equal(t, "LOCALFILE-NEW", file.LocalFileID)

	c.AssertNotCalled(t, "CreateAsset", mock.Anything, mock.Anything)
	c.AssertNotCalled(t, "Upload", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	c.AssertExpectations(t)
}
