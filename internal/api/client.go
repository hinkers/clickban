package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultBaseURL = "https://api.clickup.com/api/v2"

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

type Option func(*Client)

func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		token:   token,
		baseURL: defaultBaseURL,
		http:    &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) Get(path string, result interface{}) error {
	return c.do(http.MethodGet, path, nil, result)
}

func (c *Client) Post(path string, body interface{}, result interface{}) error {
	return c.do(http.MethodPost, path, body, result)
}

func (c *Client) Put(path string, body interface{}, result interface{}) error {
	return c.do(http.MethodPut, path, body, result)
}

func (c *Client) Delete(path string, result interface{}) error {
	return c.do(http.MethodDelete, path, nil, result)
}

func (c *Client) do(method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
