package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/pooknast/HiveMindFusion/internal/config"
)

// IngestDoc is a single document to embed and store.
type IngestDoc struct {
	Text     string `json:"text"`
	Source   string `json:"source"`
	Consumer string `json:"consumer"`
}

// IngestRequest is the JSON body for POST /admin/ingest.
// Use "documents" for batch mode or top-level "text"/"source"/"consumer" for single-doc mode.
type IngestRequest struct {
	Documents []IngestDoc `json:"documents,omitempty"`
	Text      string      `json:"text,omitempty"`
	Source    string      `json:"source,omitempty"`
	Consumer  string      `json:"consumer,omitempty"`
}

// IngestResponse is the JSON response body.
type IngestResponse struct {
	Ingested int    `json:"ingested"`
	Error    string `json:"error,omitempty"`
}

// Ingester handles the /admin/ingest endpoint.
type Ingester struct {
	cfg *config.Config
	hc  *http.Client
}

// NewIngester creates an Ingester.
func NewIngester(cfg *config.Config) *Ingester {
	return &Ingester{
		cfg: cfg,
		hc:  &http.Client{Timeout: 60 * time.Second},
	}
}

// ServeHTTP handles POST /admin/ingest.
func (ing *Ingester) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	docs, err := ing.parseRequest(r)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(docs) == 0 {
		writeJSONError(w, "no documents provided", http.StatusBadRequest)
		return
	}

	n, err := ing.ingestDocs(r.Context(), docs)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(IngestResponse{Ingested: n}) //nolint:errcheck
}

func (ing *Ingester) parseRequest(r *http.Request) ([]IngestDoc, error) {
	ct := r.Header.Get("Content-Type")

	// Multipart form (file upload).
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, fmt.Errorf("parsing multipart: %w", err)
		}
		source := r.FormValue("source")
		consumer := r.FormValue("consumer")
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, fmt.Errorf("reading file field: %w", err)
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, 32<<20))
		if err != nil {
			return nil, fmt.Errorf("reading file content: %w", err)
		}
		return []IngestDoc{{Text: string(data), Source: source, Consumer: consumer}}, nil
	}

	// JSON body.
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	var req IngestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if len(req.Documents) > 0 {
		return req.Documents, nil
	}
	if req.Text != "" {
		return []IngestDoc{{Text: req.Text, Source: req.Source, Consumer: req.Consumer}}, nil
	}
	return nil, fmt.Errorf("provide 'text' or 'documents' field")
}

func (ing *Ingester) ingestDocs(ctx context.Context, docs []IngestDoc) (int, error) {
	embedEndpoint := ing.cfg.Embed.Endpoint
	if embedEndpoint == "" {
		embedEndpoint = "http://localhost:11434"
	}
	embedModel := ing.cfg.Embed.Model
	if embedModel == "" {
		embedModel = "qwen2.5:0.5b"
	}
	collection := ing.cfg.Qdrant.Collection
	if collection == "" {
		collection = "hivemind"
	}

	type qdrantPoint struct {
		ID      uint64                 `json:"id"`
		Vector  []float32              `json:"vector"`
		Payload map[string]interface{} `json:"payload"`
	}
	type qdrantUpsert struct {
		Points []qdrantPoint `json:"points"`
	}

	points := make([]qdrantPoint, 0, len(docs))
	for _, doc := range docs {
		vec, err := ing.getEmbedding(ctx, embedEndpoint, embedModel, doc.Text)
		if err != nil {
			return 0, fmt.Errorf("embedding doc (source=%q): %w", doc.Source, err)
		}
		points = append(points, qdrantPoint{
			ID:     rand.Uint64(), //nolint:gosec
			Vector: vec,
			Payload: map[string]interface{}{
				"text":      doc.Text,
				"source":    doc.Source,
				"consumer":  doc.Consumer,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		})
	}

	payload, err := json.Marshal(qdrantUpsert{Points: points})
	if err != nil {
		return 0, fmt.Errorf("marshaling upsert: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", ing.cfg.Qdrant.Endpoint, collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("building qdrant request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ing.hc.Do(req)
	if err != nil {
		return 0, fmt.Errorf("qdrant upsert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("qdrant status %d: %s", resp.StatusCode, body)
	}

	return len(points), nil
}

type ollamaEmbedReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResp struct {
	Embedding []float32 `json:"embedding"`
}

func (ing *Ingester) getEmbedding(ctx context.Context, endpoint, model, text string) ([]float32, error) {
	payload, err := json.Marshal(ollamaEmbedReq{Model: model, Prompt: text})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/api/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ing.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama status %d: %s", resp.StatusCode, body)
	}

	var result ollamaEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding ollama response: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned by model")
	}

	return result.Embedding, nil
}

func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(IngestResponse{Error: msg}) //nolint:errcheck
}
