package usecase

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/entity"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/storage"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/rs/zerolog/log"
)

type RestoreUseCase struct {
	config         *config.Config
	client         icnk_client.Client
	store          store.Store
	downloadClient *http.Client
}

func NewRestoreUseCase(
	config *config.Config, client icnk_client.Client, store store.Store,
) *RestoreUseCase {
	return &RestoreUseCase{
		config: config,
		client: client,
		store:  store,
		// Originals can be large, so allow a generous timeout for the download.
		downloadClient: &http.Client{Timeout: 30 * time.Minute},
	}
}

// RestoreRequested polls the storage-gateway transfer queue for file sets that
// Iconik has queued to be copied ONTO the local storage (a "transfer to storage"
// in the UI), downloads each original from its signed source URL to the matching
// local path, registers the local file set, and acknowledges the transfer.
func (uc *RestoreUseCase) RestoreRequested() {
	ctx := context.Background()
	localStorageID := uc.config.Iconik.LocalStorageID

	transfers, err := uc.client.GetStorageTransfersTo(ctx, localStorageID)
	if err != nil {
		log.Error().Err(err).Str("service", "restore").Msg("Error listing pending transfers")
		return
	}

	for _, t := range transfers {
		if err := uc.handleTransfer(ctx, t); err != nil {
			log.Error().
				Err(err).
				Str("service", "restore").
				Str("transfer", t.ID).
				Msg("Error handling transfer, acking as failed")

			if ackErr := uc.client.AckStorageTransferTo(ctx, localStorageID, t.ID, false); ackErr != nil {
				log.Error().
					Err(ackErr).
					Str("service", "restore").
					Str("transfer", t.ID).
					Msg("Error acking failed transfer")
			}
			continue
		}

		if err := uc.client.AckStorageTransferTo(ctx, localStorageID, t.ID, true); err != nil {
			log.Error().
				Err(err).
				Str("service", "restore").
				Str("transfer", t.ID).
				Msg("Error acking completed transfer")
		}
	}
}

func (uc *RestoreUseCase) handleTransfer(ctx context.Context, t icnk_client.Transfer) error {
	relDir := normalizeDir(t.DestinationDirectoryPath)
	filename := t.DestinationFilename
	if filename == "" {
		filename = t.DestinationFileSetName
	}
	relPath := relDir + filename
	destPath := uc.config.Scanner.Dir + relPath

	// The exact composition of the destination fields depends on how the local
	// storage is mounted in Iconik, so log them to make live verification easy.
	log.Info().
		Str("service", "restore").
		Str("transfer", t.ID).
		Str("asset_id", t.AssetID).
		Str("dest_base_dir", t.DestinationBaseDirectory).
		Str("dest_dir", t.DestinationDirectoryPath).
		Str("dest_filename", t.DestinationFilename).
		Str("dest_path", destPath).
		Bool("add_file_set", t.AddFileSet).
		Msg("Handling transfer to local storage")

	// Pending transfers come back without a populated original_url, so resolve a
	// signed URL for the source (cloud) copy from the asset's files.
	downloadURL := t.OriginalURL
	if downloadURL == "" {
		files, err := uc.client.GetAssetFiles(ctx, t.AssetID, true)
		if err != nil {
			return fmt.Errorf("list asset files: %w", err)
		}
		downloadURL = uc.pickSourceURL(files, t)
	}
	if downloadURL == "" {
		return fmt.Errorf("could not resolve a source download URL for transfer")
	}

	if err := storage.Download(uc.downloadClient, downloadURL, destPath); err != nil {
		return fmt.Errorf("download original: %w", err)
	}

	log.Debug().
		Str("service", "restore").
		Str("transfer", t.ID).
		Str("dest_path", destPath).
		Msg("Downloaded original")

	fileSetID, fileID, err := uc.registerDownloaded(ctx, t, relDir, filename, destPath)
	if err != nil {
		return err
	}

	uc.updateStore(relPath, relDir, filename, t, fileSetID, fileID)

	return nil
}

// registerDownloaded creates the local file_set + file for a freshly-downloaded
// original and closes it so Iconik marks it present. When Iconik already created
// the destination file_set (add_file_set == false), it reuses that ID.
func (uc *RestoreUseCase) registerDownloaded(
	ctx context.Context, t icnk_client.Transfer, relDir, filename, destPath string,
) (string, string, error) {
	if !t.AddFileSet && t.FileSetID != "" {
		// Iconik owns the destination file_set; nothing to create here.
		return t.FileSetID, "", nil
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return "", "", err
	}

	fileSetName := t.DestinationFileSetName
	if fileSetName == "" {
		fileSetName = filename
	}

	fileSet, err := uc.client.CreateFileSet(
		ctx,
		t.AssetID, &icnk_client.FileSet{
			FormatID:     t.FormatID,
			StorageID:    uc.config.Iconik.LocalStorageID,
			BaseDir:      relDir,
			Name:         fileSetName,
			ComponentIds: []string{},
		},
	)
	if err != nil {
		return "", "", fmt.Errorf("create local file set: %w", err)
	}

	file, err := uc.client.CreateFile(
		ctx,
		t.AssetID,
		&icnk_client.File{
			StorageID:        uc.config.Iconik.LocalStorageID,
			FormatID:         t.FormatID,
			FileSetID:        fileSet.ID,
			Type:             "FILE",
			DirectoryPath:    relDir,
			OriginalName:     filename,
			Size:             info.Size(),
			FileDateCreated:  info.ModTime().Format(time.RFC3339),
			FileDateModified: info.ModTime().Format(time.RFC3339),
		},
	)
	if err != nil {
		return "", "", fmt.Errorf("create local file: %w", err)
	}

	if err := uc.client.CloseFile(ctx, t.AssetID, file.ID); err != nil {
		return "", "", fmt.Errorf("close local file: %w", err)
	}

	return fileSet.ID, file.ID, nil
}

func (uc *RestoreUseCase) updateStore(
	relPath, relDir, filename string, t icnk_client.Transfer, fileSetID, fileID string,
) {
	entry, err := uc.store.GetFile(relPath)
	if err != nil {
		// No prior record (e.g. a file synconik never uploaded itself).
		entry = &entity.File{
			DirectoryPath: relDir,
			Name:          filename,
			Type:          "FILE",
			AssetID:       t.AssetID,
			FormatID:      t.FormatID,
		}
	}

	entry.LocalStorageID = uc.config.Iconik.LocalStorageID
	entry.LocalFileSetID = fileSetID
	entry.LocalFileID = fileID

	if err := uc.store.SaveFile(relPath, entry); err != nil {
		log.Error().
			Err(err).
			Str("service", "restore").
			Str("path", relPath).
			Msg("Error updating store after restore")
	}
}

// pickSourceURL selects the signed download URL of the source copy for a
// transfer: preferring the file on the transfer's original storage, then any
// file not on the local storage.
func (uc *RestoreUseCase) pickSourceURL(files []icnk_client.File, t icnk_client.Transfer) string {
	for _, f := range files {
		if f.StorageID == t.OriginalStorageID && f.URL != "" {
			return f.URL
		}
	}
	for _, f := range files {
		if f.StorageID != uc.config.Iconik.LocalStorageID && f.URL != "" {
			return f.URL
		}
	}
	return ""
}

// normalizeDir strips a leading slash and ensures a non-empty directory ends in
// a trailing slash so it composes cleanly with a filename.
func normalizeDir(dir string) string {
	dir = strings.TrimPrefix(dir, "/")
	if dir != "" && !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return dir
}
