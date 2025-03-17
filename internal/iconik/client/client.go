package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	AppID      string
	Token      string
}

func NewClient(baseURL, AppID, Token string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		AppID: AppID,
		Token: Token,
	}
}

func (c *Client) NewRequest(
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

func (c *Client) Do(req *http.Request, v interface{}) error {
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
