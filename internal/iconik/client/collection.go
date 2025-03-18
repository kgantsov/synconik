package client

import (
	"context"
	"fmt"
)

type Collection struct {
	ID        string `json:"id,omitempty"`
	Title     string `json:"title"`
	ParentID  string `json:"parent_id,omitempty"`
	StorageID string `json:"storage_id,omitempty"`
}

func (c *APIClient) CreateCollection(ctx context.Context, collection *Collection) (*Collection, error) {
	req, err := c.NewRequest(
		ctx, "POST", fmt.Sprintf("/API/assets/v1/collections/"), collection,
	)
	if err != nil {
		return nil, err
	}

	var newCollection Collection
	err = c.Do(req, &newCollection)
	if err != nil {
		return nil, err
	}

	return &newCollection, nil
}
