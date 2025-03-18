package client

import (
	"context"
	"fmt"
)

const ChunkSize = 5 * 1024 * 1024 // 5 MB

type File struct {
	Name              string            `json:"name"`
	OriginalName      string            `json:"original_name"`
	DirectoryPath     string            `json:"directory_path"`
	Size              int64             `json:"size"`
	Type              string            `json:"type"`
	StorageID         string            `json:"storage_id"`
	FileSetID         string            `json:"file_set_id"`
	FormatID          string            `json:"format_id"`
	UploadURL         string            `json:"upload_url"`
	UploadCredentials map[string]string `json:"upload_credentials"`
	ID                string            `json:"id"`

	FileDateCreated  string `json:"file_date_created,omitempty"`
	FileDateModified string `json:"file_date_modified,omitempty"`
}

func (c *APIClient) CreateFile(ctx context.Context, asset_id string, file *File) (*File, error) {
	req, err := c.NewRequest(
		ctx, "POST", fmt.Sprintf("/API/files/v1/assets/%s/files/", asset_id), file,
	)
	if err != nil {
		return nil, err
	}

	var newFile File
	err = c.Do(req, &newFile)
	if err != nil {
		return nil, err
	}

	return &newFile, nil
}

func (c *APIClient) TriggerTranscodding(ctx context.Context, asset_id, file_id string) (string, error) {
	type Body struct {
		UseStorageTranscodeIgnorePattern bool `json:"use_storage_transcode_ignore_pattern"`
		Priority                         int  `json:"priority"`
	}

	body := Body{
		UseStorageTranscodeIgnorePattern: true,
		Priority:                         5,
	}

	req, err := c.NewRequest(
		ctx,
		"POST",
		fmt.Sprintf("/API/files/v1/assets/%s/files/%s/keyframes/", asset_id, file_id),
		body,
	)
	if err != nil {
		return "", err
	}

	type Transcoding struct {
		JobID string `json:"job_id"`
	}
	transcoding := Transcoding{}
	err = c.Do(req, &transcoding)
	if err != nil {
		return "", err
	}

	return transcoding.JobID, nil
}

func (c *APIClient) CloseFile(ctx context.Context, id, file_id string) error {
	type Body struct {
		Status            string `json:"status"`
		ProgressProcessed int    `json:"progress_processed"`
	}

	body := Body{
		Status:            "CLOSED",
		ProgressProcessed: 100,
	}

	req, err := c.NewRequest(
		ctx, "PATCH", fmt.Sprintf("/API/files/v1/assets/%s/files/%s/", id, file_id), body,
	)
	if err != nil {
		return fmt.Errorf("====>>>: %w", err)
	}

	err = c.Do(req, &body)
	if err != nil {
		return fmt.Errorf("error closing the file: %w", err)
	}

	return nil
}
