package client

import (
	"context"
	"fmt"
)

type Format struct {
	ID             string              `json:"id"`
	IsOnline       bool                `json:"is_online"`
	Metadata       []map[string]string `json:"metadata"`
	Name           string              `json:"name"`
	Status         string              `json:"status"`
	StorageMethods []string            `json:"storage_methods"`
}

func (c *APIClient) CreateAssetFormat(ctx context.Context, id string, format *Format) (*Format, error) {
	req, err := c.NewRequest(
		ctx, "POST", fmt.Sprintf("/API/files/v1/assets/%s/formats/", id), format,
	)
	if err != nil {
		return nil, err
	}

	var newFormat Format
	err = c.Do(req, &newFormat)
	if err != nil {
		return nil, err
	}

	return &newFormat, nil
}
