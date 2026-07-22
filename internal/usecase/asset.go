package usecase

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/entity"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/storage"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/rs/zerolog/log"
)

type AssetUseCase struct {
	config       *config.Config
	client       icnk_client.Client
	store        store.Store
	storage      *icnk_client.Storage
	localStorage *icnk_client.Storage
}

func NewAssetUseCase(
	config *config.Config,
	client icnk_client.Client,
	store store.Store,
	storage *icnk_client.Storage,
	localStorage *icnk_client.Storage,
) *AssetUseCase {
	return &AssetUseCase{
		config:       config,
		client:       client,
		store:        store,
		storage:      storage,
		localStorage: localStorage,
	}
}

func (uc *AssetUseCase) UploadIfNotExists(path string, info os.FileInfo) error {
	exists, err := uc.store.ExistsFile(path)
	if err != nil {
		log.Debug().Str("service", "asset_usecase").Msgf("File exists: %s", path)
		return err
	}

	if exists {
		return nil
	}

	file, err := uc.UploadAsset(path, info)
	if err != nil {
		return err
	}

	err = uc.store.SaveFile(path, file)
	if err != nil {
		log.Error().Err(err).Str("service", "asset_usecase").Msg("Error saving file")
	}

	return nil
}

func (uc *AssetUseCase) UploadAsset(path string, info os.FileInfo) (*entity.File, error) {
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	var netClient = &http.Client{
		Timeout:   time.Minute * 10,
		Transport: netTransport,
	}

	var iconikStorage storage.Storage
	if uc.storage.Method == "GCS" {
		iconikStorage = storage.NewGCSStorage(netClient)
	} else if uc.storage.Method == "S3" {
		iconikStorage = storage.NewS3Storage(netClient)
	} else if uc.storage.Method == "B2" {
		iconikStorage = storage.NewB2Storage(netClient)
	} else {
		return nil, fmt.Errorf("Unknown storage method: %s", uc.storage.Method)
	}

	// get directory path from path
	dirPath := ""
	if len(path) > 1 {
		dirPath = path[:len(path)-len(info.Name())]
	}

	log.Debug().Str("service", "asset_usecase").Msgf("Uploading file: %s directory: %s", path, dirPath)

	ctx := context.Background()

	f := &entity.File{
		DirectoryPath:    dirPath,
		Name:             info.Name(),
		Type:             "FILE",
		Size:             int(info.Size()),
		FileDateCreated:  info.ModTime().Format(time.RFC3339),
		FileDateModified: info.ModTime().Format(time.RFC3339),
	}

	// If the file is already present on the cloud storage, record a mapping instead
	// of re-uploading its bytes (and make sure the local mirror file set exists). A
	// lookup error aborts here rather than falling through to create a new asset,
	// which would duplicate the asset on a transient failure.
	mapped, ok, err := uc.mapExistingCloudFile(ctx, dirPath, info, f)
	if err != nil {
		return nil, err
	}
	if ok {
		return mapped, nil
	}

	// Note: the store record is written by the caller only after this whole handshake
	// succeeds. If any step below fails we deliberately persist nothing, so the file
	// is retried on the next scan (the storage dedup check above prevents a duplicate
	// upload once the bytes did land).

	asset := &icnk_client.Asset{Title: info.Name(), Status: "ACTIVE", Type: "ASSET"}

	parentDir, err := uc.store.GetFile(strings.TrimRight(dirPath, "/"))
	if err == nil {
		asset.CollectionID = parentDir.ID
	}

	asset, err = uc.client.CreateAsset(ctx, asset)
	if err != nil {
		return nil, err
	}

	f.AssetID = asset.ID

	format, err := uc.client.CreateAssetFormat(
		ctx,
		asset.ID,
		&icnk_client.Format{
			Name:           "ORIGINAL",
			Status:         "ACTIVE",
			Metadata:       []map[string]string{{"internet_media_type": "image/jpeg"}},
			StorageMethods: []string{uc.storage.Method, uc.localStorage.Method},
		},
	)

	if err != nil {
		return nil, err
	}

	f.FormatID = format.ID

	cloudDir := uc.cloudDir(dirPath)

	fileSet, err := uc.client.CreateFileSet(
		ctx,
		asset.ID, &icnk_client.FileSet{
			FormatID:     format.ID,
			StorageID:    uc.storage.ID,
			BaseDir:      cloudDir,
			Name:         info.Name(),
			ComponentIds: []string{},
		},
	)

	if err != nil {
		return nil, err
	}

	f.FileSetID = fileSet.ID

	file, err := uc.client.CreateFile(
		ctx,
		asset.ID,
		&icnk_client.File{
			StorageID:        uc.storage.ID,
			FormatID:         format.ID,
			FileSetID:        fileSet.ID,
			Type:             f.Type,
			DirectoryPath:    cloudDir,
			OriginalName:     info.Name(),
			Size:             info.Size(),
			FileDateCreated:  info.ModTime().Format(time.RFC3339),
			FileDateModified: info.ModTime().Format(time.RFC3339),
		},
	)

	if err != nil {
		return nil, err
	}

	f.ID = file.ID

	absolutePath := uc.config.Scanner.Dir + path

	err = retry.Do(
		func() error {
			err = uc.client.Upload(ctx, iconikStorage, absolutePath, file)
			if err != nil {
				return err
			}

			return nil
		},
		retry.Attempts(3),
		retry.Delay(1*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.RetryIf(func(err error) bool {
			return err != nil
		}),
	)

	if err != nil {
		log.Error().Err(err).Str("service", "asset_usecase").Msgf("Error uploading file: %s", path)
		return nil, err
	}

	err = uc.client.CloseFile(ctx, asset.ID, file.ID)
	if err != nil {
		return nil, err
	}

	_, err = uc.client.TriggerTranscoding(ctx, asset.ID, file.ID)
	if err != nil {
		return nil, err
	}

	// Register the same asset on the local ("FILE" method) storage as a second
	// file_set + file. The bytes already live on disk, so there is no upload —
	// the file is created and immediately closed to mark it present locally.
	err = uc.registerLocalFileSet(ctx, asset.ID, format.ID, dirPath, info, f)
	if err != nil {
		log.Error().Err(err).Str("service", "asset_usecase").Msgf("Error registering local file: %s", path)
		return nil, err
	}

	return f, nil
}

// registerLocalFileSet creates a file_set + file on the local ("FILE" method)
// Iconik storage for an already-existing asset/format, without uploading any
// bytes, and records the resulting IDs on f.
func (uc *AssetUseCase) registerLocalFileSet(
	ctx context.Context, assetID, formatID, dirPath string, info os.FileInfo, f *entity.File,
) error {
	fileSet, err := uc.client.CreateFileSet(
		ctx,
		assetID, &icnk_client.FileSet{
			FormatID:     formatID,
			StorageID:    uc.localStorage.ID,
			BaseDir:      dirPath,
			Name:         info.Name(),
			ComponentIds: []string{},
		},
	)
	if err != nil {
		return err
	}

	file, err := uc.client.CreateFile(
		ctx,
		assetID,
		&icnk_client.File{
			StorageID:        uc.localStorage.ID,
			FormatID:         formatID,
			FileSetID:        fileSet.ID,
			Type:             f.Type,
			DirectoryPath:    dirPath,
			OriginalName:     info.Name(),
			Size:             info.Size(),
			FileDateCreated:  info.ModTime().Format(time.RFC3339),
			FileDateModified: info.ModTime().Format(time.RFC3339),
		},
	)
	if err != nil {
		return err
	}

	if err := uc.client.CloseFile(ctx, assetID, file.ID); err != nil {
		return err
	}

	f.LocalStorageID = uc.localStorage.ID
	f.LocalFileSetID = fileSet.ID
	f.LocalFileID = file.ID

	return nil
}

// cloudDir builds the directory_path used on the cloud (B2/S3/GCS) storage from the
// relative dirPath, prefixed with the scan-root folder name (config.StoragePrefix).
// The result mirrors the local folder layout, e.g. "2026/2026-05-23/" ->
// "originals/2026/2026-05-23". Iconik rejects a leading slash and normalizes away a
// trailing one, so neither is included. The local ("FILE" method) storage keeps the
// relative dirPath and must not use this prefix.
func (uc *AssetUseCase) cloudDir(dirPath string) string {
	prefix := uc.config.StoragePrefix()
	d := strings.Trim(dirPath, "/")
	if d == "" {
		return prefix
	}
	return prefix + "/" + d
}

// mapExistingCloudFile checks whether the file already exists on the cloud storage.
// When it does, it fills f with the discovered Iconik IDs, ensures the asset has a
// local ("FILE" method) file set so restore/deletion sync keeps working, and returns
// (f, true, nil) so the caller can persist the mapping without uploading any bytes.
// A lookup error is returned so the caller aborts: creating a new asset on an
// ambiguous failure would duplicate an asset that may already exist. A confirmed
// empty result returns (nil, false, nil) to signal a genuine new upload.
func (uc *AssetUseCase) mapExistingCloudFile(
	ctx context.Context, dirPath string, info os.FileInfo, f *entity.File,
) (*entity.File, bool, error) {
	files, err := uc.client.GetStorageFiles(ctx, uc.storage.ID, uc.cloudDir(dirPath))
	if err != nil {
		log.Warn().
			Err(err).
			Str("service", "asset_usecase").
			Msgf("Error checking if file exists on storage: %s", info.Name())
		return nil, false, fmt.Errorf("check existing cloud file %s: %w", info.Name(), err)
	}

	// Iconik's storage `name` can differ from the on-disk original (it may append
	// the file id, e.g. "_DSC7627_<id>.jpg"), so match on original_name too.
	var existing *icnk_client.File
	for i := range files {
		if files[i].OriginalName == info.Name() || files[i].Name == info.Name() {
			existing = &files[i]
			break
		}
	}
	if existing == nil {
		return nil, false, nil
	}

	log.Info().
		Str("service", "asset_usecase").
		Str("asset_id", existing.AssetID).
		Msgf("File already exists on storage, recording mapping instead of uploading: %s", info.Name())

	f.AssetID = existing.AssetID
	f.FormatID = existing.FormatID
	f.FileSetID = existing.FileSetID
	f.StorageID = existing.StorageID
	f.ID = existing.ID

	// Register the local mirror file set only if the asset does not already have one.
	if !uc.hasLocalFileSet(ctx, existing.AssetID) {
		if err := uc.registerLocalFileSet(
			ctx, existing.AssetID, existing.FormatID, dirPath, info, f,
		); err != nil {
			log.Error().
				Err(err).
				Str("service", "asset_usecase").
				Msgf("Error registering local file set for existing asset: %s", info.Name())
		}
	}

	return f, true, nil
}

// hasLocalFileSet reports whether the asset already has a live file on the local
// ("FILE" method) storage. Deleting a file set is a soft delete on Iconik's side, so
// the old file lingers with a DELETED status and must be ignored — otherwise a
// re-added file would never get its local file set re-registered. A lookup error is
// logged and treated as "no local file set".
func (uc *AssetUseCase) hasLocalFileSet(ctx context.Context, assetID string) bool {
	files, err := uc.client.GetAssetFiles(ctx, assetID, false)
	if err != nil {
		log.Warn().
			Err(err).
			Str("service", "asset_usecase").
			Msgf("Error listing files for asset %s", assetID)
		return false
	}
	for _, file := range files {
		if file.StorageID != uc.localStorage.ID {
			continue
		}
		log.Debug().
			Str("service", "asset_usecase").
			Str("asset_id", assetID).
			Str("file_id", file.ID).
			Str("status", file.Status).
			Msg("Found file on local storage")
		if file.Status != "DELETED" {
			return true
		}
	}
	return false
}
