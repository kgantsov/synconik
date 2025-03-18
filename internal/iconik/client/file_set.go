package client

import (
	"context"
	"fmt"
)

type FileSet struct {
	FormatID     string   `json:"format_id"`
	StorageID    string   `json:"storage_id"`
	BaseDir      string   `json:"base_dir"`
	Name         string   `json:"name"`
	ComponentIds []string `json:"component_ids"`
	ID           string   `json:"id"`
}

func (c *APIClient) CreateFileSet(ctx context.Context, id string, fileSet *FileSet) (*FileSet, error) {
	req, err := c.NewRequest(
		ctx, "POST", fmt.Sprintf("/API/files/v1/assets/%s/file_sets/", id), fileSet,
	)
	if err != nil {
		return nil, err
	}

	var newFileSet FileSet
	err = c.Do(req, &newFileSet)
	if err != nil {
		return nil, err
	}

	return &newFileSet, nil
}
