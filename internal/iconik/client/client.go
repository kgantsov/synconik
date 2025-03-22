package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kgantsov/synconik/internal/storage"
)

type Client interface {
	CreateAsset(ctx context.Context, asset *Asset) (*Asset, error)

	CreateCollection(ctx context.Context, collection *Collection) (*Collection, error)

	CreateFileSet(ctx context.Context, id string, fileSet *FileSet) (*FileSet, error)

	CreateFile(ctx context.Context, asset_id string, file *File) (*File, error)
	TriggerTranscodding(ctx context.Context, asset_id, file_id string) (string, error)
	CloseFile(ctx context.Context, id, file_id string) error

	CreateAssetFormat(ctx context.Context, id string, format *Format) (*Format, error)

	GetStorage(ctx context.Context, id string) (*Storage, error)
	Upload(ctx context.Context, storage storage.Storage, filePath string, file *File) error
}

type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
	AppID      string
	Token      string
}

func NewClient(httpClient *http.Client, baseURL, AppID, Token string) *APIClient {
	return &APIClient{
		BaseURL:    baseURL,
		HTTPClient: httpClient,
		AppID:      AppID,
		Token:      Token,
	}
}

func (c *APIClient) NewRequest(
	ctx context.Context, method, url string, body interface{},
) (*http.Request, error) {
	var buf bytes.Buffer
	if body != nil {
		err := json.NewEncoder(&buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+url, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("App-ID", c.AppID)
	req.Header.Set("Auth-Token", c.Token)
	return req, nil
}

func (c *APIClient) Do(req *http.Request, v interface{}) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("request failed: %s", bodyBytes)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
