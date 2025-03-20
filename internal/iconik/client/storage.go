package client

import (
	"context"
	"fmt"

	"github.com/kgantsov/synconik/internal/entity"
	"github.com/kgantsov/synconik/internal/storage"
)

type Storage struct {
	ID       string                 `json:"id,omitempty"`
	Name     string                 `json:"name"`
	Method   string                 `json:"method"`
	Purpose  string                 `json:"purpose"`
	Status   string                 `json:"status"`
	Settings map[string]interface{} `json:"settings"`
}

func (c *APIClient) GetStorage(ctx context.Context, id string) (*Storage, error) {
	req, err := c.NewRequest(ctx, "GET", fmt.Sprintf("/API/files/v1/storages/%s/", id), nil)
	if err != nil {
		return nil, err
	}

	var storage Storage
	err = c.Do(req, &storage)
	if err != nil {
		return nil, err
	}

	return &storage, nil
}

func (c *APIClient) Upload(ctx context.Context, storage storage.Storage, filePath string, file *File) error {
	return storage.Upload(filePath, &entity.UploadFile{
		Name:              file.Name,
		OriginalName:      file.OriginalName,
		DirectoryPath:     file.DirectoryPath,
		Size:              file.Size,
		Type:              file.Type,
		StorageID:         file.StorageID,
		FileSetID:         file.FileSetID,
		FormatID:          file.FormatID,
		UploadURL:         file.UploadURL,
		UploadCredentials: file.UploadCredentials,
		ID:                file.ID,
		FileDateCreated:   file.FileDateCreated,
		FileDateModified:  file.FileDateModified,
	})
}
