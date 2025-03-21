package usecase

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/iconik/client"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUploadAsset(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	dir := t.TempDir()

	imagePath := filepath.Join(dir, "image.jpg")
	err := os.WriteFile(imagePath, []byte("test"), 0644)
	assert.NoError(t, err)

	imageFileInfo, err := os.Stat(imagePath)
	assert.NoError(t, err)

	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Dir:      dir,
			Interval: 10,
		},
	}

	client := client.NewMockClient()
	storage := &icnk_client.Storage{
		ID:      "240E2FF6-0215-4F10-A0A4-37366C0F710B",
		Name:    "test",
		Method:  "S3",
		Purpose: "FILES",
		Status:  "ACTIVE",
	}
	assetUseCase := NewAssetUseCase(cfg, client, store, storage)

	client.On("CreateAsset", mock.Anything, &icnk_client.Asset{
		Title:  imageFileInfo.Name(),
		Status: "ACTIVE",
		Type:   "ASSET",
	}).Return(&icnk_client.Asset{
		ID: "47265105-BE2B-4C3F-8997-66BAB2893D0D",
	}, nil)

	client.On("CreateAssetFormat", mock.Anything, "47265105-BE2B-4C3F-8997-66BAB2893D0D", &icnk_client.Format{
		Name:           "ORIGINAL",
		Status:         "ACTIVE",
		Metadata:       []map[string]string{{"internet_media_type": "image/jpeg"}},
		StorageMethods: []string{"S3"},
	}).Return(&icnk_client.Format{
		ID: "EDEF4933-4CB5-4FFE-B55F-C00549AC164B",
	}, nil)

	client.On("CreateFileSet", mock.Anything, "47265105-BE2B-4C3F-8997-66BAB2893D0D", &icnk_client.FileSet{
		FormatID:     "EDEF4933-4CB5-4FFE-B55F-C00549AC164B",
		StorageID:    "240E2FF6-0215-4F10-A0A4-37366C0F710B",
		BaseDir:      dir + "/",
		Name:         imageFileInfo.Name(),
		ComponentIds: []string{},
	}).Return(&icnk_client.FileSet{
		ID: "05BE6FD5-9B15-4C7D-8B54-5749239A89D4",
	}, nil)

	fileInfo := &icnk_client.File{
		ID:            "D025605F-CF64-4EE5-9F48-E6DD5D363473",
		Name:          imageFileInfo.Name(),
		OriginalName:  imageFileInfo.Name(),
		DirectoryPath: dir,
		Size:          imageFileInfo.Size(),
		Type:          "image/jpeg",
		FormatID:      "EDEF4933-4CB5-4FFE-B55F-C00549AC164B",
		FileSetID:     "05BE6FD5-9B15-4C7D-8B54-5749239A89D4",
		UploadURL:     "https://test.com",
		UploadCredentials: map[string]string{
			"access_key": "test",
			"secret_key": "test",
		},
	}
	client.On("CreateFile", mock.Anything, "47265105-BE2B-4C3F-8997-66BAB2893D0D", &icnk_client.File{
		OriginalName:     imageFileInfo.Name(),
		DirectoryPath:    dir + "/",
		Size:             imageFileInfo.Size(),
		Type:             "FILE",
		StorageID:        "240E2FF6-0215-4F10-A0A4-37366C0F710B",
		FormatID:         "EDEF4933-4CB5-4FFE-B55F-C00549AC164B",
		FileSetID:        "05BE6FD5-9B15-4C7D-8B54-5749239A89D4",
		FileDateCreated:  imageFileInfo.ModTime().Format(time.RFC3339),
		FileDateModified: imageFileInfo.ModTime().Format(time.RFC3339),
	}).Return(fileInfo, nil)

	client.On("Upload", mock.Anything, mock.Anything, mock.Anything, fileInfo).Return(nil)

	client.On(
		"CloseFile",
		mock.Anything,
		"47265105-BE2B-4C3F-8997-66BAB2893D0D",
		"D025605F-CF64-4EE5-9F48-E6DD5D363473",
	).Return(nil)
	client.On(
		"TriggerTranscodding",
		mock.Anything,
		"47265105-BE2B-4C3F-8997-66BAB2893D0D",
		"D025605F-CF64-4EE5-9F48-E6DD5D363473",
	).Return("C2BC2D18-FBF5-4D89-92B3-35E586ABCCD8", nil)

	file, err := assetUseCase.UploadAsset(imagePath, imageFileInfo)
	assert.NoError(t, err)
	assert.Equal(t, file.ID, "D025605F-CF64-4EE5-9F48-E6DD5D363473")
}
