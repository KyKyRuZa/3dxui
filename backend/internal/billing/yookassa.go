package billing

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to the YooKassa Payments API (v3).
type Client struct {
	shopID     string
	secretKey  string
	apiURL     string
	httpClient *http.Client
}

// New builds a YooKassa client. apiURL is typically https://api.yookassa.ru/v3.
func New(shopID, secretKey, apiURL string) *Client {
	return &Client{
		shopID:     shopID,
		secretKey:  secretKey,
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) authHeader() string {
	raw := c.shopID + ":" + c.secretKey
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// Amount is the monetary value of a payment.
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// Payment is the subset of the YooKassa payment object we care about.
type Payment struct {
	ID           string            `json:"id"`
	Status       string            `json:"status"`
	Paid         bool              `json:"paid"`
	Amount       Amount            `json:"amount"`
	Description  string            `json:"description"`
	Metadata     map[string]string `json:"metadata"`
	Confirmation struct {
		Type            string `json:"type"`
		ConfirmationURL string `json:"confirmation_url"`
	} `json:"confirmation"`
}

// CreatePayment requests a new payment and returns the created object.
// idempotencyKey must be unique per attempt; amountValue is a formatted string
// like "299.00". metadata is echoed back in the webhook for correlation.
func (c *Client) CreatePayment(idempotencyKey, amountValue, currency, description, returnURL string, metadata map[string]string) (*Payment, error) {
	body := map[string]interface{}{
		"amount": map[string]string{
			"value":    amountValue,
			"currency": currency,
		},
		"capture":     true,
		"description": description,
		"metadata":    metadata,
		"confirmation": map[string]string{
			"type":       "redirect",
			"return_url": returnURL,
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.apiURL+"/payments", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Idempotence-Key", idempotencyKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("yookassa create payment: status %d body %s", resp.StatusCode, string(respBody))
	}
	var p Payment
	if err := json.Unmarshal(respBody, &p); err != nil {
		return nil, fmt.Errorf("yookassa decode: %w", err)
	}
	return &p, nil
}

// GetPayment fetches a payment by id, used to verify a webhook notification
// against the API (YooKassa does not sign webhooks, so re-fetching is the
// authoritative way to confirm a payment actually succeeded).
func (c *Client) GetPayment(id string) (*Payment, error) {
	req, err := http.NewRequest(http.MethodGet, c.apiURL+"/payments/"+id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yookassa get payment: status %d body %s", resp.StatusCode, string(respBody))
	}
	var p Payment
	if err := json.Unmarshal(respBody, &p); err != nil {
		return nil, fmt.Errorf("yookassa decode: %w", err)
	}
	return &p, nil
}

// GenIdempotencyKey returns a random hex string suitable for the
// Idempotence-Key header.
func GenIdempotencyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
