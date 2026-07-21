package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
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

// DeleteFileSet removes a file_set (its file entries and, for uploadable
// storages, the actual bytes) from an asset. Used to drop the local file_set
// when a file disappears from disk, leaving the asset and its cloud copy intact.
// immediately=true skips the recycle bin and purges it outright so it doesn't
// linger in Iconik's deleted state.
func (c *APIClient) DeleteFileSet(ctx context.Context, asset_id, file_set_id string) error {
	req, err := c.NewRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("/API/files/v1/assets/%s/file_sets/%s/?immediately=true", asset_id, file_set_id),
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error deleting file set: %s", bodyBytes)
	}

	return nil
}
