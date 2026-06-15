package pii

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ScanRequest is the payload sent to PII Shield.
type ScanRequest struct {
	AgentID   string `json:"agent_id"`
	Direction string `json:"direction"`
	Text      string `json:"text"`
}

// ScanResponse is the response from PII Shield.
type ScanResponse struct {
	Decision      string `json:"decision"`
	SanitizedText string `json:"sanitized_text"`
}

// Client wraps the PII Shield HTTP endpoint with a circuit breaker.
// The circuit opens when PII Shield has been unreachable for >cbTimeout and
// routes traffic according to bypassOnFailure.
type Client struct {
	endpoint        string
	bypassOnFailure bool
	http            *http.Client
	cbTimeout       time.Duration

	mu           sync.Mutex
	firstFailure time.Time // zero when healthy
}

// NewClient constructs a PII Shield client from the given config values.
func NewClient(endpoint string, timeoutMs int, bypassOnFailure bool) *Client {
	return &Client{
		endpoint:        endpoint,
		bypassOnFailure: bypassOnFailure,
		http: &http.Client{
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
		},
		cbTimeout: 30 * time.Second,
	}
}

// Scan sends text to PII Shield and returns the scan result.
// When the circuit breaker is open it returns allow (bypass) or an error (block).
func (c *Client) Scan(agentID, direction, text string) (*ScanResponse, error) {
	if c.isOpen() {
		if c.bypassOnFailure {
			return &ScanResponse{Decision: "allow", SanitizedText: text}, nil
		}
		return nil, fmt.Errorf("pii: circuit breaker open")
	}

	body, err := json.Marshal(ScanRequest{
		AgentID:   agentID,
		Direction: direction,
		Text:      text,
	})
	if err != nil {
		return nil, fmt.Errorf("pii: marshal request: %w", err)
	}

	resp, err := c.http.Post(c.endpoint+"/scan", "application/json", bytes.NewReader(body))
	if err != nil {
		c.recordFailure()
		if c.bypassOnFailure {
			return &ScanResponse{Decision: "allow", SanitizedText: text}, nil
		}
		return nil, fmt.Errorf("pii: shield unreachable: %w", err)
	}
	defer resp.Body.Close()

	c.recordSuccess()

	var result ScanResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("pii: decode response: %w", err)
	}
	return &result, nil
}

// isOpen returns true when PII Shield has been unreachable for >= cbTimeout.
func (c *Client) isOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.firstFailure.IsZero() {
		return false
	}
	return time.Since(c.firstFailure) >= c.cbTimeout
}

func (c *Client) recordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.firstFailure.IsZero() {
		c.firstFailure = time.Now()
	}
}

func (c *Client) recordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.firstFailure = time.Time{}
}
