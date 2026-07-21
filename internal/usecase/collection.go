package usecase

import (
	"context"
	"os"
	"strings"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/entity"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/rs/zerolog/log"
)

type CollectionUseCase struct {
	config *config.Config
	client icnk_client.Client
	store  store.Store
}

func NewCollectionUseCase(
	config *config.Config, client icnk_client.Client, store store.Store,
) *CollectionUseCase {
	return &CollectionUseCase{
		config: config,
		client: client,
		store:  store,
	}
}

func (uc *CollectionUseCase) CreateCollectionIfNotExists(path string, info os.FileInfo) error {
	exists, err := uc.store.ExistsFile(path)
	if err != nil {
		return err
	}

	if exists {
		log.Debug().Str("service", "collection_usecase").Msgf("Collection %s already exists", path)

		return nil
	}

	ctx := context.Background()

	dirPath := ""
	if len(path) > 1 {
		dirPath = path[:len(path)-len(info.Name())]
	}

	// Scan root (relativePath == ""): when a root collection_id is configured, treat
	// that existing collection as the root directory itself — map the scan folder to
	// it instead of creating a new nested collection. Children then resolve it as
	// their parent. Without a collection_id, fall through and create a collection for
	// the scan folder as before.
	if path == "" && uc.config.Iconik.CollectionID != "" {
		log.Info().
			Str("service", "collection_usecase").
			Str("collection_id", uc.config.Iconik.CollectionID).
			Msg("Using configured root collection as the scan-root directory")
		return uc.saveCollection(path, dirPath, info.Name(), uc.config.Iconik.CollectionID)
	}

	collection := &icnk_client.Collection{
		Title: info.Name(),
	}

	parentDir, err := uc.store.GetFile(strings.TrimRight(dirPath, "/"))
	if err == nil {
		collection.ParentID = parentDir.ID
	}

	// If a collection with this title already exists under the parent in Iconik (e.g.
	// the local store was wiped), reuse it instead of creating a duplicate.
	if existing, ok := uc.findExistingCollection(ctx, collection.ParentID, info.Name()); ok {
		log.Info().
			Str("service", "collection_usecase").
			Str("collection_id", existing.ID).
			Msgf("Collection already exists in Iconik, recording mapping: %s", path)
		return uc.saveCollection(path, dirPath, info.Name(), existing.ID)
	}

	collection, err = uc.client.CreateCollection(ctx, collection)
	if err != nil {
		return err
	}

	return uc.saveCollection(path, dirPath, info.Name(), collection.ID)
}

// findExistingCollection looks for a sub-collection of parentID whose title matches,
// so an already-existing Iconik collection is reused rather than duplicated. It uses
// the search API (filtered to direct children of parentID) instead of paginating
// through every child. It can only search under a known parent; at the true root (no
// parent, no collection_id) it returns false and the caller creates the collection. A
// lookup error is logged and treated as "not found" so it never blocks collection
// creation. Because the search query is fuzzy, results are confirmed with an exact
// title match.
func (uc *CollectionUseCase) findExistingCollection(
	ctx context.Context, parentID, title string,
) (*icnk_client.Collection, bool) {
	if parentID == "" {
		return nil, false
	}
	subs, err := uc.client.SearchCollections(ctx, parentID, title)
	if err != nil {
		log.Warn().
			Err(err).
			Str("service", "collection_usecase").
			Msgf("Error searching sub-collections of %s", parentID)
		return nil, false
	}
	for i := range subs {
		if subs[i].Title == title {
			return &subs[i], true
		}
	}
	return nil, false
}

func (uc *CollectionUseCase) saveCollection(path, dirPath, name, id string) error {
	return uc.store.SaveFile(path, &entity.File{
		DirectoryPath: dirPath,
		Name:          name,
		Type:          "directory",
		ID:            id,
	})
}
