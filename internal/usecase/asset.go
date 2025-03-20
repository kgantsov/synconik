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
	config  *config.Config
	client  icnk_client.Client
	store   store.Store
	storage *icnk_client.Storage
}

func NewAssetUseCase(
	config *config.Config, client icnk_client.Client, store store.Store, storage *icnk_client.Storage,
) *AssetUseCase {
	return &AssetUseCase{
		config:  config,
		client:  client,
		store:   store,
		storage: storage,
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

	f := &entity.File{
		DirectoryPath:    dirPath,
		Name:             info.Name(),
		Type:             "FILE",
		Size:             int(info.Size()),
		FileDateCreated:  info.ModTime().Format(time.RFC3339),
		FileDateModified: info.ModTime().Format(time.RFC3339),
	}

	err := uc.store.SaveFile(path, f)
	if err != nil {
		log.Error().Err(err).Str("service", "asset_usecase").Msg("Error saving file")
	}

	asset := &icnk_client.Asset{Title: info.Name(), Status: "ACTIVE", Type: "ASSET"}

	parentDir, err := uc.store.GetFile(strings.TrimRight(dirPath, "/"))
	if err == nil {
		asset.CollectionID = parentDir.ID
	}

	ctx := context.Background()

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
			StorageMethods: []string{uc.storage.Method},
		},
	)

	if err != nil {
		return nil, err
	}

	f.FormatID = format.ID

	fileSet, err := uc.client.CreateFileSet(
		ctx,
		asset.ID, &icnk_client.FileSet{
			FormatID:     format.ID,
			StorageID:    uc.storage.ID,
			BaseDir:      dirPath,
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
			DirectoryPath:    dirPath,
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

	_, err = uc.client.TriggerTranscodding(ctx, asset.ID, file.ID)
	if err != nil {
		return nil, err
	}

	return f, nil
}
