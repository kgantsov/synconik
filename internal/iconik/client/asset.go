package client

import (
	"context"

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

func (c *APIClient) CreateAsset(ctx context.Context, asset *Asset) (*Asset, error) {
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
