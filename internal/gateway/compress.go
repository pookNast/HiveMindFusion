package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/pook/hivemind/internal/config"
)

// Compressor calls the headroom-srv sidecar to compress request bodies.
type Compressor struct {
	endpoint    string
	minBodySize int
	client      *http.Client
}

// NewCompressor creates a compressor from config. Returns nil if disabled.
func NewCompressor(cfg config.Compression) *Compressor {
	if !cfg.Enabled {
		return nil
	}
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 500 * time.Millisecond
	}
	minSize := cfg.MinBodySize
	if minSize == 0 {
		minSize = 100
	}
	return &Compressor{
		endpoint:    cfg.Endpoint,
		minBodySize: minSize,
		client:      &http.Client{Timeout: timeout},
	}
}

type compressRequest struct {
	Body   json.RawMessage `json:"body"`
	Target string          `json:"target"`
}

type compressResponse struct {
	Body     json.RawMessage `json:"body"`
	Metadata struct {
		Action        string   `json:"action"`
		OriginalChars int      `json:"original_chars"`
		CompressedChars int    `json:"compressed_chars"`
		SavingsPct    float64  `json:"savings_pct"`
		StagesRun     []string `json:"stages_run"`
		ElapsedMs     float64  `json:"elapsed_ms"`
	} `json:"metadata"`
}

// CompressBody sends the request body to headroom-srv for compression.
// Returns the (possibly compressed) body. On any error, returns original body.
func (c *Compressor) CompressBody(body []byte) []byte {
	if c == nil {
		return body
	}

	// Skip small bodies
	if len(body) < c.minBodySize {
		return body
	}

	reqPayload := compressRequest{
		Body:   body,
		Target: "messages",
	}
	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		log.Printf("[hivemind] compress: marshal error: %v", err)
		return body
	}

	resp, err := c.client.Post(c.endpoint+"/compress", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		log.Printf("[hivemind] compress: sidecar call failed: %v", err)
		return body
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[hivemind] compress: sidecar returned %d: %s", resp.StatusCode, string(respBody)[:min(200, len(respBody))])
		return body
	}

	var result compressResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[hivemind] compress: decode error: %v", err)
		return body
	}

	if result.Metadata.Action == "compressed" && len(result.Body) > 0 {
		log.Printf("[hivemind] compress: %d -> %d bytes (%.1f%% savings) in %.1fms, stages=%v",
			result.Metadata.OriginalChars, result.Metadata.CompressedChars,
			result.Metadata.SavingsPct, result.Metadata.ElapsedMs,
			result.Metadata.StagesRun)
		return result.Body
	}

	return body
}
