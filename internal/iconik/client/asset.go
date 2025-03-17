package client

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

type Asset struct {
	ExternalID   string `json:"external_id"`
	ID           string `json:"id"`
	IsBlocked    bool   `json:"is_blocked"`
	IsOnline     bool   `json:"is_online"`
	Status       string `json:"status"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	CollectionID string `json:"collection_id,omitempty"`
}

func (c *Client) GetAsset(ctx context.Context, id string) (*Asset, error) {
	req, err := c.NewRequest(ctx, "GET", fmt.Sprintf("/API/assets/v1/assets/%s/", id), nil)
	if err != nil {
		return nil, err
	}

	var asset Asset
	err = c.Do(req, &asset)
	if err != nil {
		return nil, err
	}

	return &asset, nil
}

func (c *Client) CreateAsset(ctx context.Context, asset *Asset) (*Asset, error) {
	req, err := c.NewRequest(ctx, "POST", "/API/assets/v1/assets/?assign_to_collection=True", asset)
	if err != nil {
		log.Error().Str("service", "iconik_client").Err(err).Msg("Error creating asset request")
		return nil, err
	}

	var newAsset Asset
	err = c.Do(req, &newAsset)
	if err != nil {
		log.Error().Err(err).Str("service", "iconik_client").Msg("Error creating asset")
		return nil, err
	}

	return &newAsset, nil
}
