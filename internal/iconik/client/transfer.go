package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Transfer is a pending storage-gateway transfer of a file set TO a local
// ("FILE" method) storage, as returned by GetStorageTransfersTo. It carries a
// signed original_url for downloading the source bytes plus the destination
// location on the local storage.
type Transfer struct {
	ID                       string `json:"id"`
	AssetID                  string `json:"asset_id"`
	FileSetID                string `json:"file_set_id"`
	FormatID                 string `json:"format_id"`
	OriginalStorageID        string `json:"original_storage_id"`
	OriginalURL              string `json:"original_url"`
	LocalStorageID           string `json:"local_storage_id"`
	DestinationBaseDirectory string `json:"destination_base_directory"`
	DestinationDirectoryPath string `json:"destination_directory_path"`
	DestinationFilename      string `json:"destination_filename"`
	DestinationFileSetName   string `json:"destination_file_set_name"`
	AddFileSet               bool   `json:"add_file_set"`
}

// GetStorageTransfersTo returns the pending transfers of file sets TO the given
// local storage (i.e. files that Iconik has queued to be pulled down onto it).
func (c *APIClient) GetStorageTransfersTo(ctx context.Context, storage_id string) ([]Transfer, error) {
	req, err := c.NewRequest(
		ctx,
		"GET",
		fmt.Sprintf("/API/files/v1/storages/%s/transfers_to/", storage_id),
		nil,
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Objects []Transfer `json:"objects"`
	}
	if err := c.Do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Objects, nil
}

// AckStorageTransferTo acknowledges a handled transfer, marking it completed on
// success or failed otherwise, which removes it from the pending queue.
func (c *APIClient) AckStorageTransferTo(ctx context.Context, storage_id, transfer_id string, success bool) error {
	outcome := "failed=true"
	if success {
		outcome = "completed=true"
	}

	req, err := c.NewRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("/API/files/v1/storages/%s/transfers_to/%s/?%s", storage_id, transfer_id, outcome),
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
		return fmt.Errorf("error acknowledging transfer: %s", bodyBytes)
	}

	return nil
}
