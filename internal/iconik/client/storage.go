package client

import (
	"context"
	"fmt"
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
