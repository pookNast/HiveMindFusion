// Package rag provides optional RAG (Retrieval-Augmented Generation) context injection.
// It queries Qdrant for semantically relevant documents based on the user's message,
// then prepends the results as a system message to the chat request.
//
// Disabled by default — opt-in per consumer via ConsumerConfig.Enabled.
package rag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ConsumerConfig holds per-consumer RAG settings.
// All fields default to safe off-state (Enabled: false).
type ConsumerConfig struct {
	// Enabled controls whether RAG injection is active. Default: false.
	Enabled bool

	// Collection is the Qdrant collection to search (e.g. "knowledge_graph", "obsidian_notes").
	Collection string

	// TopK is the maximum number of results to retrieve.
	TopK int

	// MinScore is the minimum cosine similarity score [0,1] to include a result.
	MinScore float64

	// EmbedEndpoint is an OpenAI-compatible /v1/embeddings endpoint used to vectorise
	// the user message (e.g. "http://localhost:11434/v1").
	EmbedEndpoint string

	// EmbedModel is the embedding model name passed to the embed endpoint.
	EmbedModel string
}

// maxContextChars caps the injected context block size to bound prompt inflation.
const maxContextChars = 8192

// Middleware injects Qdrant-retrieved context into OpenAI-compatible chat requests.
type Middleware struct {
	qdrant     *QdrantClient
	cfg        ConsumerConfig
	httpClient *http.Client
}

// NewMiddleware constructs a Middleware. qdrantEndpoint is the Qdrant base URL.
// cfg controls per-consumer behaviour; if cfg.Enabled is false, InjectContext is a no-op.
func NewMiddleware(qdrantEndpoint string, cfg ConsumerConfig) *Middleware {
	return &Middleware{
		qdrant: NewQdrantClient(qdrantEndpoint),
		cfg:    cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// chatRequest is the subset of an OpenAI chat completion request that we inspect.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   *bool         `json:"stream,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// InjectContext reads an OpenAI-compatible chat request body, retrieves relevant
// documents from Qdrant, and prepends a system message containing that context.
// If RAG is disabled or any step fails, the original body is returned unchanged.
func (m *Middleware) InjectContext(body []byte) ([]byte, error) {
	if !m.cfg.Enabled {
		return body, nil
	}

	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		// Not a valid chat request; pass through.
		return body, nil
	}

	userText := lastUserMessage(req.Messages)
	if userText == "" {
		return body, nil
	}

	vector, err := m.embed(userText)
	if err != nil {
		// Embedding failed — skip RAG injection, don't break the request.
		return body, nil
	}

	collection := m.cfg.Collection
	if collection == "" {
		collection = "knowledge_graph"
	}
	topK := m.cfg.TopK
	if topK <= 0 {
		topK = 5
	}

	results, err := m.qdrant.Search(collection, vector, topK, m.cfg.MinScore)
	if err != nil || len(results) == 0 {
		return body, nil
	}

	contextBlock := buildContextBlock(results)
	// ponytail: hardcoded cap — upgrade: per-consumer config if limits ever diverge
	if len(contextBlock) > maxContextChars {
		contextBlock = contextBlock[:maxContextChars] + "\n</retrieved_context>\n[... context truncated ...]"
	}
	req.Messages = prependSystemContext(req.Messages, contextBlock)

	out, err := json.Marshal(req)
	if err != nil {
		return body, nil
	}
	return out, nil
}

// embed calls an OpenAI-compatible /v1/embeddings endpoint and returns the vector.
func (m *Middleware) embed(text string) ([]float32, error) {
	if m.cfg.EmbedEndpoint == "" {
		return nil, fmt.Errorf("rag: embed_endpoint not configured")
	}

	payload := map[string]string{
		"model": m.cfg.EmbedModel,
		"input": text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(m.cfg.EmbedEndpoint, "/") + "/v1/embeddings"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rag: embed endpoint returned %d: %s", resp.StatusCode, b)
	}

	var embedResp struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, err
	}
	if len(embedResp.Data) == 0 || len(embedResp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("rag: empty embedding in response")
	}

	return embedResp.Data[0].Embedding, nil
}

// lastUserMessage returns the content of the last message with role "user".
func lastUserMessage(messages []chatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

// ponytail: only strips our own delimiters — upgrade: strip ChatML tokens (<|im_start|>) if a model-specific prompt-template injection is observed
var contextSanitizer = strings.NewReplacer(
	"<retrieved_context>", "[blocked]",
	"</retrieved_context>", "[blocked]",
)

// buildContextBlock formats retrieved Qdrant results as a readable context string,
// wrapped in explicit delimiters so a poisoned KB entry cannot close the block
// early and inject arbitrary prompt content.
func buildContextBlock(results []SearchResult) string {
	var b strings.Builder
	b.WriteString("<retrieved_context>\n")
	b.WriteString("Relevant context retrieved from knowledge base:\n\n")
	for i, r := range results {
		text := contextSanitizer.Replace(extractText(r.Payload))
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "[%d] (score %.3f)\n%s\n\n", i+1, r.Score, text)
	}
	b.WriteString("</retrieved_context>")
	return b.String()
}

// extractText pulls a human-readable string from a Qdrant payload.
// It tries common field names used by knowledge_graph and obsidian_notes collections.
func extractText(payload map[string]interface{}) string {
	for _, key := range []string{"content", "text", "body", "chunk", "page_content"} {
		if v, ok := payload[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// prependSystemContext inserts or augments the system message at position 0.
func prependSystemContext(messages []chatMessage, context string) []chatMessage {
	if context == "" {
		return messages
	}
	if len(messages) > 0 && messages[0].Role == "system" {
		messages[0].Content = context + "\n\n" + messages[0].Content
		return messages
	}
	return append([]chatMessage{{Role: "system", Content: context}}, messages...)
}
