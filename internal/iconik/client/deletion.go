package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// FileDeletion is a pending deletion record on a local storage's deletion queue
// (FileDeletionFromLocalStorageSchema). Unlike the gateway "DELETE" event — which
// is only a doorbell and carries no payload — these records carry the real data
// needed to remove a file: the path (DirectoryPath + Filename) and the Iconik
// ids. ID is the deletion-record id, used to ack the record once handled.
type FileDeletion struct {
	ID            string `json:"id"`
	AssetID       string `json:"asset_id"`
	DirectoryPath string `json:"directory_path"`
	FileID        string `json:"file_id"`
	Filename      string `json:"filename"`
	FileType      string `json:"file_type"`
	FormatID      string `json:"format_id"`
	StorageID     string `json:"storage_id"`
	VersionID     string `json:"version_id"`
	JobID         string `json:"job_id"`
	KeepSource    bool   `json:"keep_source"`
}

// GetStorageDeletions returns the pending file deletions queued for a local
// storage. Iconik purges its own file document before queueing the record, so
// this queue — not the gateway event — is the source of truth for what to
// remove from disk.
func (c *APIClient) GetStorageDeletions(ctx context.Context, storage_id string) ([]FileDeletion, error) {
	req, err := c.NewRequest(
		ctx,
		"GET",
		fmt.Sprintf("/API/files/v1/storages/%s/deletions/", storage_id),
		nil,
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Objects []FileDeletion `json:"objects"`
	}
	if err := c.Do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Objects, nil
}

// DeleteStorageDeletion acks a handled deletion record, removing it from the
// storage's deletion queue so it is not returned again.
func (c *APIClient) DeleteStorageDeletion(ctx context.Context, storage_id, deletion_id string) error {
	req, err := c.NewRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("/API/files/v1/storages/%s/deletions/%s/", storage_id, deletion_id),
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
		return fmt.Errorf("error acknowledging deletion record: %s", bodyBytes)
	}

	return nil
}
