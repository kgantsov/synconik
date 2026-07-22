package usecase

import (
	"context"
	"os"

	"github.com/kgantsov/synconik/internal/config"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/rs/zerolog/log"
)

type ReconcileUseCase struct {
	config *config.Config
	client icnk_client.Client
	store  store.Store
}

func NewReconcileUseCase(
	config *config.Config, client icnk_client.Client, store store.Store,
) *ReconcileUseCase {
	return &ReconcileUseCase{
		config: config,
		client: client,
		store:  store,
	}
}

// ReconcileDeletions walks the store and, for any FILE whose bytes have
// disappeared from disk, removes only the local ("FILE" method) file_set + file
// from Iconik. The asset and its cloud copy are left intact so the original can be
// requested back later. The store record is dropped so the next scan treats the
// path as a cache miss: if the file reappears on disk, UploadAsset re-discovers the
// still-present cloud copy and re-registers a local file set on the same asset.
func (uc *ReconcileUseCase) ReconcileDeletions() {
	entries, err := uc.store.ListFiles()
	if err != nil {
		log.Error().Err(err).Str("service", "reconcile").Msg("Error listing stored files")
		return
	}

	ctx := context.Background()

	for _, entry := range entries {
		f := entry.File

		// Only files that are actually registered on local storage are candidates.
		if f.Type != "FILE" || f.LocalFileSetID == "" {
			continue
		}

		absolutePath := uc.config.Scanner.Dir + entry.Path

		if _, err := os.Stat(absolutePath); err == nil {
			continue // still present on disk
		} else if !os.IsNotExist(err) {
			log.Error().
				Err(err).
				Str("service", "reconcile").
				Msgf("Error stating %s", absolutePath)
			continue
		}

		log.Info().
			Str("service", "reconcile").
			Str("path", entry.Path).
			Msg("File deleted from disk, removing local file set")

		if err := uc.client.DeleteFileSet(ctx, f.AssetID, f.LocalFileSetID); err != nil {
			log.Error().
				Err(err).
				Str("service", "reconcile").
				Str("path", entry.Path).
				Msg("Error deleting local file set")
			continue
		}

		// Drop the store record entirely. On the next scan the file is a cache miss;
		// if it reappears on disk, mapExistingCloudFile finds the still-present cloud
		// copy (by directory_path + name) and re-registers a local file set on the
		// same asset instead of creating a duplicate.
		if err := uc.store.DeleteFile(entry.Path); err != nil {
			log.Error().
				Err(err).
				Str("service", "reconcile").
				Str("path", entry.Path).
				Msg("Error deleting store record after local delete")
		}
	}
}
