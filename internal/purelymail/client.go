// Package purelymail implements a thin client for the PurelyMail API
// (https://news.purelymail.com/api/index.html). All endpoints are POST under
// /api/v0, authentication uses the Purelymail-Api-Token header, and responses
// are wrapped as {"result": <T>}.
package purelymail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const DefaultBaseURL = "https://purelymail.com"

// APIError is the error payload returned by the API on failure.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("purelymail: %s: %s", e.Code, e.Message)
}

// Client talks to the PurelyMail API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{},
	}
}

func (c *Client) do(ctx context.Context, path string, req any, out any) error {
	var body io.Reader
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("purelymail: encode request: %w", err)
		}
		body = bytes.NewReader(b)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v0/"+path, body)
	if err != nil {
		return fmt.Errorf("purelymail: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Purelymail-Api-Token", c.token)

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("purelymail: %s: %w", path, err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("purelymail: %s: read response: %w", path, err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var apiErr APIError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Code != "" {
			return fmt.Errorf("purelymail: %s: %w", path, apiErr)
		}
		return fmt.Errorf("purelymail: %s: HTTP %d: %s", path, httpResp.StatusCode, string(respBody))
	}

	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("purelymail: %s: decode response: %w", path, err)
	}
	if out != nil {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return fmt.Errorf("purelymail: %s: decode result: %w", path, err)
		}
	}
	return nil
}

// CheckAccountCredit returns the current account credit.
func (c *Client) CheckAccountCredit(ctx context.Context) (credit string, err error) {
	var out struct {
		Credit string `json:"credit"`
	}
	if err = c.do(ctx, "checkAccountCredit", struct{}{}, &out); err != nil {
		return "", err
	}
	return out.Credit, nil
}

// ListUser lists all usernames under the account (up to 1000).
func (c *Client) ListUser(ctx context.Context) ([]string, error) {
	var out struct {
		Users []string `json:"users"`
	}
	if err := c.do(ctx, "listUser", struct{}{}, &out); err != nil {
		return nil, err
	}
	return out.Users, nil
}

// CreateUser creates a user under the account's domain.
func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) error {
	return c.do(ctx, "createUser", req, nil)
}

// GetUser fetches a single user's settings.
func (c *Client) GetUser(ctx context.Context, req GetUserRequest) (*GetUserResponse, error) {
	var out GetUserResponse
	if err := c.do(ctx, "getUser", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ModifyUser modifies a user's settings.
func (c *Client) ModifyUser(ctx context.Context, req ModifyUserRequest) error {
	return c.do(ctx, "modifyUser", req, nil)
}

// DeleteUser deletes a user (full username, e.g. user@domain.com).
func (c *Client) DeleteUser(ctx context.Context, req DeleteUserRequest) error {
	return c.do(ctx, "deleteUser", req, nil)
}
