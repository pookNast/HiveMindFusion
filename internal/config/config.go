package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

const defaultConfigPath = "/etc/hivemind/config.toml"

// ValidationError is returned when a required config field is missing or invalid.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config: field %q: %s", e.Field, e.Message)
}

// Gateway configures the main proxy listener ports.
type Gateway struct {
	Port                   int `toml:"port"`
	AdminPort              int `toml:"admin_port"`
	MetricsPort            int `toml:"metrics_port"`
	HealthIntervalSecs     int `toml:"health_interval_secs"`
	HealthFailureThreshold int `toml:"health_failure_threshold"`
}

// Backend defines a single upstream LLM backend.
type Backend struct {
	Name           string `toml:"name"`
	URL            string `toml:"url"`
	Model          string `toml:"model"`
	Priority       int    `toml:"priority"`
	HealthEndpoint string `toml:"health_endpoint"`
	PathRewrite    string `toml:"path_rewrite"`
	APIKey         string `toml:"api_key"`
	APIKeyEnv      string `toml:"api_key_env"`
	ExpectedDown   bool   `toml:"expected_down"`
}

// PII configures the PII Shield proxy.
type PII struct {
	Endpoint        string `toml:"endpoint"`
	Enabled         bool   `toml:"enabled"`
	BypassOnFailure bool   `toml:"bypass_on_failure"`
	TimeoutMs       int    `toml:"timeout_ms"`
}

// Models configures local model storage.
type Models struct {
	Dir     string `toml:"dir"`
	Default string `toml:"default"`
}

// Qdrant configures the vector store connection.
type Qdrant struct {
	Endpoint   string `toml:"endpoint"`
	Collection string `toml:"collection"`
}

// Embed configures the local embedding model via Ollama.
type Embed struct {
	Endpoint string `toml:"endpoint"` // Ollama base URL, e.g. http://localhost:11434
	Model    string `toml:"model"`    // e.g. qwen2.5:0.5b
}

// ConsumerLimit configures rate limits for a specific named consumer.
type ConsumerLimit struct {
	RequestsPerMinute int `toml:"requests_per_minute"`
	Burst             int `toml:"burst"`
}

// RateLimitConfig configures global defaults and per-consumer overrides.
type RateLimitConfig struct {
	DefaultRequestsPerMinute int                      `toml:"default_requests_per_minute"`
	DefaultBurst             int                      `toml:"default_burst"`
	Consumers                map[string]ConsumerLimit `toml:"consumers"`
}

// ConsumerConfig holds API key → consumer name mappings for auth.
type ConsumerConfig struct {
	APIKeys map[string]string `toml:"api_keys"`
}

// RAGConsumer configures per-consumer RAG context injection.
type RAGConsumer struct {
	Enabled    bool    `toml:"enabled"`
	Collection string  `toml:"collection"`
	TopK       int     `toml:"top_k"`
	MinScore   float64 `toml:"min_score"`
}

// RAG configures the RAG context injection middleware.
type RAG struct {
	Consumers map[string]RAGConsumer `toml:"consumers"`
}

// Compression configures the headroom-srv compression sidecar.
type Compression struct {
	Enabled      bool   `toml:"enabled"`
	Endpoint     string `toml:"endpoint"`
	MinBodySize  int    `toml:"min_body_size"`
	TimeoutMs    int    `toml:"timeout_ms"`
}

// FusionPanel defines a single fusion tier's model roster.
type FusionPanel struct {
	Panelists   []string `toml:"panelists"`
	Judge       string   `toml:"judge"`
	Synth       string   `toml:"synth"`
	Deliberator string   `toml:"deliberator"`
	TimeoutMs   int      `toml:"timeout_ms"`
	MinQuorum   int      `toml:"min_quorum"`
}

// Fusion configures the in-process multi-model orchestrator.
type Fusion struct {
	Enabled bool                    `toml:"enabled"`
	Panels  map[string]FusionPanel  `toml:"panels"`
}

// Config is the top-level configuration structure.
type Config struct {
	Gateway   Gateway         `toml:"gateway"`
	Backends  []Backend       `toml:"backends"`
	PII       PII             `toml:"pii"`
	Models    Models          `toml:"models"`
	RateLimit RateLimitConfig `toml:"rate_limit"`
	Consumers ConsumerConfig  `toml:"consumers"`
	Qdrant      Qdrant       `toml:"qdrant"`
	Embed       Embed        `toml:"embed"`
	RAG         RAG          `toml:"rag"`
	Compression Compression  `toml:"compression"`
	Fusion      Fusion       `toml:"fusion"`
}

// Load reads the config from HIVEMIND_CONFIG env var or the default path.
func Load() (*Config, error) {
	path := os.Getenv("HIVEMIND_CONFIG")
	if path == "" {
		path = defaultConfigPath
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("config: failed to decode %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.resolveEnvVars()

	return &cfg, nil
}

// resolveEnvVars expands env-var references in config fields.
func (c *Config) resolveEnvVars() {
	for i := range c.Backends {
		if c.Backends[i].APIKeyEnv != "" && c.Backends[i].APIKey == "" {
			c.Backends[i].APIKey = os.Getenv(c.Backends[i].APIKeyEnv)
		}
	}
}

func (c *Config) validate() error {
	var errs []error

	if c.Gateway.Port == 0 {
		errs = append(errs, &ValidationError{Field: "gateway.port", Message: "required"})
	}
	if c.Gateway.AdminPort == 0 {
		errs = append(errs, &ValidationError{Field: "gateway.admin_port", Message: "required"})
	}
	if c.Gateway.MetricsPort == 0 {
		errs = append(errs, &ValidationError{Field: "gateway.metrics_port", Message: "required"})
	}

	if len(c.Backends) == 0 {
		errs = append(errs, &ValidationError{Field: "backends", Message: "at least one backend required"})
	}
	for i, b := range c.Backends {
		if b.Name == "" {
			errs = append(errs, &ValidationError{Field: fmt.Sprintf("backends[%d].name", i), Message: "required"})
		}
		if b.URL == "" {
			errs = append(errs, &ValidationError{Field: fmt.Sprintf("backends[%d].url", i), Message: "required"})
		}
		if b.Model == "" {
			errs = append(errs, &ValidationError{Field: fmt.Sprintf("backends[%d].model", i), Message: "required"})
		}
	}

	if c.PII.Enabled && c.PII.Endpoint == "" {
		errs = append(errs, &ValidationError{Field: "pii.endpoint", Message: "required when pii.enabled is true"})
	}
	if c.PII.Enabled && c.PII.TimeoutMs == 0 {
		errs = append(errs, &ValidationError{Field: "pii.timeout_ms", Message: "required when pii.enabled is true"})
	}

	if c.Qdrant.Endpoint == "" {
		errs = append(errs, &ValidationError{Field: "qdrant.endpoint", Message: "required"})
	}

	if c.Fusion.Enabled {
		if len(c.Fusion.Panels) == 0 {
			errs = append(errs, &ValidationError{Field: "fusion.panels", Message: "at least one panel required when fusion is enabled"})
		}
		for name, p := range c.Fusion.Panels {
			if len(p.Panelists) == 0 {
				errs = append(errs, &ValidationError{Field: fmt.Sprintf("fusion.panels.%s.panelists", name), Message: "required"})
			}
			if p.Judge == "" {
				errs = append(errs, &ValidationError{Field: fmt.Sprintf("fusion.panels.%s.judge", name), Message: "required"})
			}
			if p.Synth == "" {
				errs = append(errs, &ValidationError{Field: fmt.Sprintf("fusion.panels.%s.synth", name), Message: "required"})
			}
			if p.TimeoutMs == 0 {
				errs = append(errs, &ValidationError{Field: fmt.Sprintf("fusion.panels.%s.timeout_ms", name), Message: "required"})
			}
		}
	}

	return errors.Join(errs...)
}
