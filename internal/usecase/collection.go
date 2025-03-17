package usecase

import (
	"context"
	"os"
	"strings"

	"github.com/kgantsov/synconic/internal/config"
	"github.com/kgantsov/synconic/internal/entity"
	icnk_client "github.com/kgantsov/synconic/internal/iconik/client"
	"github.com/kgantsov/synconic/internal/store"
	"github.com/rs/zerolog/log"
)

type CollectionUseCase struct {
	config *config.Config
	client *icnk_client.Client
	store  store.Store
}

func NewCollectionUseCase(
	config *config.Config, client *icnk_client.Client, store store.Store,
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

	collection := &icnk_client.Collection{
		Title: info.Name(),
	}

	parentDir, err := uc.store.GetFile(strings.TrimRight(dirPath, "/"))
	if err == nil {
		collection.ParentID = parentDir.ID
	}

	collection, err = uc.client.CreateCollection(ctx, collection)
	if err != nil {
		return err
	}

	file := &entity.File{
		DirectoryPath: dirPath,
		Name:          info.Name(),
		Type:          "directory",
		ID:            collection.ID,
	}

	err = uc.store.SaveFile(path, file)
	if err != nil {
		return err
	}

	return nil
}
