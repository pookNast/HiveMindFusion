package rag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// QdrantClient is a minimal HTTP client for Qdrant vector search.
type QdrantClient struct {
	endpoint   string
	httpClient *http.Client
}

// NewQdrantClient creates a client targeting the given Qdrant endpoint (e.g. "http://localhost:6333").
func NewQdrantClient(endpoint string) *QdrantClient {
	return &QdrantClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SearchResult is a single point returned from a Qdrant search.
type SearchResult struct {
	ID      interface{}            `json:"id"`
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

type qdrantSearchRequest struct {
	Vector     []float32 `json:"vector"`
	Limit      int       `json:"limit"`
	WithPayload bool     `json:"with_payload"`
	ScoreThreshold *float64 `json:"score_threshold,omitempty"`
}

type qdrantSearchResponse struct {
	Result []SearchResult `json:"result"`
	Status string         `json:"status"`
}

// Search queries a Qdrant collection using the provided dense vector.
// Results below minScore are filtered out. topK limits the number of results.
func (q *QdrantClient) Search(collection string, vector []float32, topK int, minScore float64) ([]SearchResult, error) {
	payload := qdrantSearchRequest{
		Vector:      vector,
		Limit:       topK,
		WithPayload: true,
	}
	if minScore > 0 {
		payload.ScoreThreshold = &minScore
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("rag: marshal search request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", q.endpoint, collection)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("rag: build search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rag: qdrant search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rag: qdrant returned %d: %s", resp.StatusCode, b)
	}

	var result qdrantSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("rag: decode search response: %w", err)
	}

	return result.Result, nil
}
