package panel

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	username   string
	password   string
	apiToken   string
	httpClient *http.Client
	loggedIn   bool
}

func New(baseURL, username, password, apiToken string) *Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		username:   username,
		password:   password,
		apiToken:   apiToken,
		httpClient: &http.Client{Transport: tr, Timeout: 15 * time.Second},
	}
}

func (c *Client) usingToken() bool {
	return c.apiToken != ""
}

func (c *Client) ensureLoggedIn(ctx context.Context) error {
	if c.loggedIn {
		return nil
	}
	if c.usingToken() {
		c.loggedIn = true
		return nil
	}
	if err := c.Login(ctx); err != nil {
		return err
	}
	c.loggedIn = true
	return nil
}

func (c *Client) Login(ctx context.Context) error {
	u := fmt.Sprintf("%s/login", c.baseURL)
	body := strings.NewReader(fmt.Sprintf(`{"username":"%s","password":"%s","twoFactorCode":""}`, c.username, c.password))
	req, err := http.NewRequestWithContext(ctx, "POST", u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("panel login: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) AddClient(ctx context.Context, email string, totalGB int64, expiryTime int64, inboundIDs []int) error {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return err
	}
	u := fmt.Sprintf("%s/panel/api/clients/add", c.baseURL)
	payload := map[string]interface{}{
		"client": map[string]interface{}{
			"email":      email,
			"totalGB":    totalGB,
			"expiryTime": expiryTime,
			"tgId":       0,
			"limitIp":    0,
			"enable":     true,
		},
		"inboundIds": inboundIDs,
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("panel AddClient: %s", string(respBody))
	}
	return nil
}

type ClientInfo struct {
	Email      string `json:"email"`
	SubID      string `json:"subId"`
	InboundIDs []int  `json:"inboundIds"`
}

func (c *Client) GetClient(ctx context.Context, email string) (*ClientInfo, error) {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/panel/api/clients/get/%s", c.baseURL, url.PathEscape(email))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("panel GetClient: %s", string(respBody))
	}
	var wrapper struct {
		Success bool        `json:"success"`
		Obj     *ClientInfo `json:"obj"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, err
	}
	if wrapper.Obj == nil {
		return nil, fmt.Errorf("panel GetClient: empty obj")
	}
	return wrapper.Obj, nil
}

func (c *Client) GetLinks(ctx context.Context, email string) ([]string, error) {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/panel/api/clients/links/%s", c.baseURL, url.PathEscape(email))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("panel GetLinks: %s", string(respBody))
	}
	var result struct {
		Obj []string `json:"obj"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return result.Obj, nil
}

func (c *Client) GetSubLinks(ctx context.Context, subID string) ([]string, error) {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/panel/api/clients/subLinks/%s", c.baseURL, url.PathEscape(subID))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("panel GetSubLinks: %s", string(respBody))
	}
	var result struct {
		Obj []string `json:"obj"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return result.Obj, nil
}

func (c *Client) AddToGroup(ctx context.Context, emails []string, group string) error {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return err
	}
	u := fmt.Sprintf("%s/panel/api/clients/groups/bulkAdd", c.baseURL)
	payload := map[string]interface{}{
		"emails": emails,
		"group":  group,
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("panel AddToGroup: %s", string(respBody))
	}
	return nil
}

func (c *Client) RemoveFromGroup(ctx context.Context, emails []string) error {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return err
	}
	u := fmt.Sprintf("%s/panel/api/clients/groups/bulkRemove", c.baseURL)
	payload := map[string]interface{}{
		"emails": emails,
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("panel RemoveFromGroup: %s", string(respBody))
	}
	return nil
}

func (c *Client) setAuthHeader(req *http.Request) {
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
}
