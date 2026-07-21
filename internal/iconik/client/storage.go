package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

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

// DeleteEnabled reports whether the storage's settings allow the gateway to
// delete files (the "delete" flag in Iconik's storage settings).
func (s *Storage) DeleteEnabled() bool {
	if s.Settings == nil {
		return false
	}
	enabled, _ := s.Settings["delete"].(bool)
	return enabled
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

// GetStorageFiles lists every file a storage holds under the given directory path
// (matched against Iconik's `directory_path`). It is used to detect a file that is
// already present on the cloud storage so it is not re-uploaded. The caller matches
// individual files by `original_name`, because Iconik's `name` filter matches the
// mangled storage name (it appends the file id, e.g. "_DSC7627_<id>.jpg") and never
// the original filename. Results are paginated.
func (c *APIClient) GetStorageFiles(
	ctx context.Context, storageID, directoryPath string,
) ([]File, error) {
	var all []File

	for page := 1; ; page++ {
		req, err := c.NewRequest(
			ctx, "GET", fmt.Sprintf("/API/files/v1/storages/%s/files/", storageID), nil,
		)
		if err != nil {
			return nil, err
		}

		q := url.Values{}
		if directoryPath != "" {
			q.Set("directory_path", directoryPath)
		}
		q.Set("per_page", "1000")
		q.Set("page", strconv.Itoa(page))
		req.URL.RawQuery = q.Encode()

		var resp struct {
			Objects []File `json:"objects"`
			Pages   int    `json:"pages"`
		}
		if err := c.Do(req, &resp); err != nil {
			return nil, err
		}

		all = append(all, resp.Objects...)
		if len(resp.Objects) == 0 || page >= resp.Pages {
			break
		}
	}

	return all, nil
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
		UploadMethod:      file.UploadMethod,
		UploadFilename:    file.UploadFilename,
		UploadURL:         file.UploadURL,
		UploadCredentials: file.UploadCredentials,
		ID:                file.ID,
		FileDateCreated:   file.FileDateCreated,
		FileDateModified:  file.FileDateModified,
	})
}
