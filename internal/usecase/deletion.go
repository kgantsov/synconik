package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgantsov/synconik/internal/config"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/rs/zerolog/log"
)

type DeletionUseCase struct {
	config *config.Config
	client icnk_client.Client
	store  store.Store
}

func NewDeletionUseCase(
	config *config.Config, client icnk_client.Client, store store.Store,
) *DeletionUseCase {
	return &DeletionUseCase{
		config: config,
		client: client,
		store:  store,
	}
}

// ProcessDeletions polls the local storage's deletion queue and removes the
// corresponding files from disk, then acks each record so it drops off the
// queue. The gateway DELETE event is only a doorbell that carries no payload;
// the deletion queue is the source of truth (see isg-events-deletion-workflow.md),
// so we poll it directly on the ticker instead of the events channel.
// Deletion only runs when the local storage's "delete" setting is enabled.
func (uc *DeletionUseCase) ProcessDeletions() {
	ctx := context.Background()
	storageID := uc.config.Iconik.LocalStorageID

	storage, err := uc.client.GetStorage(ctx, storageID)
	if err != nil {
		log.Error().Err(err).Str("service", "deletion").Msg("Error getting local storage")
		return
	}
	if !storage.DeleteEnabled() {
		log.Debug().Str("service", "deletion").Msg("Local storage delete flag is off, skipping deletions")
		return
	}

	deletions, err := uc.client.GetStorageDeletions(ctx, storageID)
	if err != nil {
		log.Error().Err(err).Str("service", "deletion").Msg("Error listing storage deletions")
		return
	}
	if len(deletions) == 0 {
		return
	}

	// Correlate deletion records to on-disk files via our own store, keyed by the
	// Iconik local file id. The record's `filename` is Iconik's storage name (it
	// appends the file id, e.g. "_DSC7627_<id>.jpg"), which does NOT match the
	// original name on disk — so we must resolve the real path from the store.
	files, err := uc.store.ListFiles()
	if err != nil {
		log.Error().Err(err).Str("service", "deletion").Msg("Error listing stored files")
		return
	}
	byLocalFileID := make(map[string]store.FileEntry, len(files))
	for _, e := range files {
		if e.File != nil && e.File.LocalFileID != "" {
			byLocalFileID[e.File.LocalFileID] = e
		}
	}

	for _, d := range deletions {
		if err := uc.handleDeletion(d, byLocalFileID); err != nil {
			log.Error().Err(err).Str("service", "deletion").Str("deletion", d.ID).Msg("Error handling deletion")
			continue // leave the record queued so it can be inspected / retried
		}

		if err := uc.client.DeleteStorageDeletion(ctx, storageID, d.ID); err != nil {
			log.Error().Err(err).Str("service", "deletion").Str("deletion", d.ID).Msg("Error acking deletion record")
		}
	}
}

func (uc *DeletionUseCase) handleDeletion(
	d icnk_client.FileDeletion, byLocalFileID map[string]store.FileEntry,
) error {
	entry, ok := byLocalFileID[d.FileID]
	if !ok {
		// Not a file we manage locally (or already reconciled). Ack it so the
		// queue drains instead of replaying the record every tick.
		log.Warn().
			Str("service", "deletion").
			Str("deletion", d.ID).
			Str("file_id", d.FileID).
			Str("filename", d.Filename).
			Msg("No local file matches deletion record, acking")
		return nil
	}

	relPath := entry.Path
	destPath := filepath.Clean(uc.config.Scanner.Dir + relPath)

	// Safety guard: never delete anything outside the scan root.
	root := filepath.Clean(uc.config.Scanner.Dir)
	if destPath == root || !strings.HasPrefix(destPath, root+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to delete path outside scan dir: %s", destPath)
	}

	// keep_source means Iconik wants the on-disk original left in place (e.g. a
	// pure metadata/file-set removal); only clear our local bookkeeping.
	if d.KeepSource {
		log.Info().
			Str("service", "deletion").
			Str("deletion", d.ID).
			Str("path", relPath).
			Msg("Deletion with keep_source, leaving file on disk")
	} else {
		log.Info().
			Str("service", "deletion").
			Str("deletion", d.ID).
			Str("path", relPath).
			Str("dest", destPath).
			Msg("Deleting local file (file set removed in Iconik)")

		if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove file: %w", err)
		}
	}

	// Keep the store record + cloud IDs so the original can be transferred back
	// later; just clear the local IDs to reflect the file is gone locally.
	entry.File.LocalStorageID = ""
	entry.File.LocalFileSetID = ""
	entry.File.LocalFileID = ""
	if err := uc.store.SaveFile(relPath, entry.File); err != nil {
		log.Error().Err(err).Str("service", "deletion").Str("path", relPath).Msg("Error updating store after local delete")
	}

	return nil
}
