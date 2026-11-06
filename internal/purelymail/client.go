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
	"strings"
	"time"
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
		// Trim a trailing slash so baseURL+"/api/v0/..." never doubles up.
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
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
	defer func() { _ = httpResp.Body.Close() }()

	// Cap the response size so a misbehaving server cannot exhaust memory.
	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 10<<20))
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
		Type   string          `json:"type"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("purelymail: %s: decode response: %w", path, err)
	}
	// The API reports logical errors as HTTP 200 with type "error".
	if envelope.Type != "" && envelope.Type != "success" {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Code != "" {
			return fmt.Errorf("purelymail: %s: %w", path, apiErr)
		}
		return fmt.Errorf("purelymail: %s: API error type %q: %s", path, envelope.Type, string(respBody))
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

// AddDomain adds a domain to the account.
func (c *Client) AddDomain(ctx context.Context, req AddDomainRequest) error {
	return c.do(ctx, "addDomain", req, nil)
}

// GetOwnershipCode fetches the TXT ownership-proof code for a domain.
func (c *Client) GetOwnershipCode(ctx context.Context, req GetOwnershipCodeRequest) (*GetOwnershipCodeResponse, error) {
	var out GetOwnershipCodeResponse
	if err := c.do(ctx, "getOwnershipCode", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateDomainSettings updates settings for a domain.
func (c *Client) UpdateDomainSettings(ctx context.Context, req UpdateDomainSettingsRequest) error {
	return c.do(ctx, "updateDomainSettings", req, nil)
}

// DeleteDomain deletes a domain.
func (c *Client) DeleteDomain(ctx context.Context, req DeleteDomainRequest) error {
	return c.do(ctx, "deleteDomain", req, nil)
}

// FindDomain fetches a single domain by name; returns nil if not present.
func (c *Client) FindDomain(ctx context.Context, name string) (*ApiDomainInfo, error) {
	domains, err := c.ListDomains(ctx, ListDomainsRequest{})
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		if d.Name == name {
			return &d, nil
		}
	}
	return nil, nil
}

// ListDomains lists domains accessible to the account.
func (c *Client) ListDomains(ctx context.Context, req ListDomainsRequest) ([]ApiDomainInfo, error) {
	var out struct {
		Domains []ApiDomainInfo `json:"domains"`
	}
	if err := c.do(ctx, "listDomains", req, &out); err != nil {
		return nil, err
	}
	return out.Domains, nil
}

// CreateRoutingRule creates a routing rule for a domain.
func (c *Client) CreateRoutingRule(ctx context.Context, req CreateRoutingRuleRequest) error {
	return c.do(ctx, "createRoutingRule", req, nil)
}

// ListRoutingRules lists all routing rules under the account.
func (c *Client) ListRoutingRules(ctx context.Context) ([]RoutingRule, error) {
	var out struct {
		Rules []RoutingRule `json:"rules"`
	}
	if err := c.do(ctx, "listRoutingRules", struct{}{}, &out); err != nil {
		return nil, err
	}
	return out.Rules, nil
}

// FindRoutingRule fetches a single routing rule by id; returns nil if absent.
func (c *Client) FindRoutingRule(ctx context.Context, id int64) (*RoutingRule, error) {
	rules, err := c.ListRoutingRules(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range rules {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, nil
}

// DeleteRoutingRule deletes a routing rule by id.
func (c *Client) DeleteRoutingRule(ctx context.Context, req DeleteRoutingRuleRequest) error {
	return c.do(ctx, "deleteRoutingRule", req, nil)
}

// CreateAppPassword creates an app password and returns its full value.
func (c *Client) CreateAppPassword(ctx context.Context, req CreateAppPasswordRequest) (*CreateAppPasswordResponse, error) {
	var out CreateAppPasswordResponse
	if err := c.do(ctx, "createAppPassword", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAppPassword deletes an app password by its full value.
func (c *Client) DeleteAppPassword(ctx context.Context, req DeleteAppPasswordRequest) error {
	return c.do(ctx, "deleteAppPassword", req, nil)
}

// ListPasswordReset lists password reset methods for a user.
func (c *Client) ListPasswordReset(ctx context.Context, req GetUserRequest) ([]PasswordResetMethod, error) {
	var out struct {
		Users []PasswordResetMethod `json:"users"`
	}
	if err := c.do(ctx, "listPasswordReset", req, &out); err != nil {
		return nil, err
	}
	return out.Users, nil
}

// UpsertPasswordReset creates or updates a password reset method.
func (c *Client) UpsertPasswordReset(ctx context.Context, req UpsertPasswordResetRequest) error {
	return c.do(ctx, "upsertPasswordReset", req, nil)
}

// DeletePasswordReset deletes a password reset method by target.
func (c *Client) DeletePasswordReset(ctx context.Context, req DeletePasswordResetRequest) error {
	return c.do(ctx, "deletePasswordReset", req, nil)
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
