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

type searchTerm struct {
	Name    string   `json:"name"`
	ValueIn []string `json:"value_in"`
}

type searchFilter struct {
	Operator string       `json:"operator"`
	Terms    []searchTerm `json:"terms"`
}

type searchRequest struct {
	DocTypes      []string     `json:"doc_types"`
	IncludeFields []string     `json:"include_fields"`
	Query         string       `json:"query"`
	SearchFields  []string     `json:"search_fields"`
	Filter        searchFilter `json:"filter"`
}

// SearchCollections finds the direct sub-collections of parentID whose title matches
// query, using Iconik's search API. This replaces paginating through every child of a
// collection: a single request returns the candidates. Filtering on `in_collections`
// restricts results to direct children (as opposed to `ancestor_collections`, which
// would also match nested descendants). Iconik search is Elasticsearch-backed, so the
// query is wrapped in double quotes to request an exact phrase match; results are still
// confirmed with an exact title comparison by the caller since the match is analyzed.
func (c *APIClient) SearchCollections(ctx context.Context, parentID, query string) ([]Collection, error) {
	body := searchRequest{
		DocTypes:      []string{"collections"},
		IncludeFields: []string{"id", "title", "parent_id"},
		Query:         fmt.Sprintf("%q", query),
		SearchFields:  []string{"title"},
		Filter: searchFilter{
			Operator: "AND",
			Terms: []searchTerm{
				{Name: "in_collections", ValueIn: []string{parentID}},
				{Name: "status", ValueIn: []string{"ACTIVE"}},
			},
		},
	}

	req, err := c.NewRequest(
		ctx, "POST", fmt.Sprintf("/API/search/v1/search/?page=1&per_page=100"), body,
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Objects []Collection `json:"objects"`
	}
	if err := c.Do(req, &resp); err != nil {
		return nil, err
	}

	return resp.Objects, nil
}
