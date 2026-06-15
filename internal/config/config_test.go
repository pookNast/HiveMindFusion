package config

import (
	"os"
	"path/filepath"
	"testing"
)

const validTOML = `
[gateway]
port         = 8400
admin_port   = 8401
metrics_port = 9090

[[backends]]
name            = "ollama-primary"
url             = "http://localhost:11434"
model           = "glm-flash:latest"
priority        = 1
health_endpoint = "/api/tags"

[pii]
endpoint          = "http://localhost:5100/scan"
enabled           = true
bypass_on_failure = false
timeout_ms        = 500

[models]
dir     = "/var/lib/hivemind/models"
default = "glm-flash:latest"

[rate_limit]
default_requests_per_minute = 60
default_burst                = 10

[rate_limit.consumers]
  [rate_limit.consumers.premium]
  requests_per_minute = 300
  burst               = 30

[qdrant]
endpoint = "http://localhost:6333"
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeTempConfig(t, validTOML)
	t.Setenv("HIVEMIND_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Gateway
	if cfg.Gateway.Port != 8400 {
		t.Errorf("gateway.port = %d, want 8400", cfg.Gateway.Port)
	}
	if cfg.Gateway.AdminPort != 8401 {
		t.Errorf("gateway.admin_port = %d, want 8401", cfg.Gateway.AdminPort)
	}
	if cfg.Gateway.MetricsPort != 9090 {
		t.Errorf("gateway.metrics_port = %d, want 9090", cfg.Gateway.MetricsPort)
	}

	// Backends
	if len(cfg.Backends) != 1 {
		t.Fatalf("backends count = %d, want 1", len(cfg.Backends))
	}
	if cfg.Backends[0].Name != "ollama-primary" {
		t.Errorf("backends[0].name = %q, want %q", cfg.Backends[0].Name, "ollama-primary")
	}
	if cfg.Backends[0].HealthEndpoint != "/api/tags" {
		t.Errorf("backends[0].health_endpoint = %q, want %q", cfg.Backends[0].HealthEndpoint, "/api/tags")
	}

	// PII
	if !cfg.PII.Enabled {
		t.Error("pii.enabled = false, want true")
	}
	if cfg.PII.TimeoutMs != 500 {
		t.Errorf("pii.timeout_ms = %d, want 500", cfg.PII.TimeoutMs)
	}

	// Models
	if cfg.Models.Default != "glm-flash:latest" {
		t.Errorf("models.default = %q, want %q", cfg.Models.Default, "glm-flash:latest")
	}

	// RateLimit
	if cfg.RateLimit.DefaultRequestsPerMinute != 60 {
		t.Errorf("rate_limit.default_requests_per_minute = %d, want 60", cfg.RateLimit.DefaultRequestsPerMinute)
	}
	if cfg.RateLimit.DefaultBurst != 10 {
		t.Errorf("rate_limit.default_burst = %d, want 10", cfg.RateLimit.DefaultBurst)
	}
	if cfg.RateLimit.Consumers["premium"].RequestsPerMinute != 300 {
		t.Errorf("rate_limit.consumers.premium.requests_per_minute = %d, want 300", cfg.RateLimit.Consumers["premium"].RequestsPerMinute)
	}
	if cfg.RateLimit.Consumers["premium"].Burst != 30 {
		t.Errorf("rate_limit.consumers.premium.burst = %d, want 30", cfg.RateLimit.Consumers["premium"].Burst)
	}

	// Qdrant
	if cfg.Qdrant.Endpoint != "http://localhost:6333" {
		t.Errorf("qdrant.endpoint = %q, want %q", cfg.Qdrant.Endpoint, "http://localhost:6333")
	}
}

func TestLoad_DefaultPath(t *testing.T) {
	// Unset env var — should fall back to /etc/hivemind/config.toml
	t.Setenv("HIVEMIND_CONFIG", "")
	_, err := Load()
	if err == nil {
		t.Skip("default config exists on this system")
	}
	// Should get a decode error (file not found), not a validation error
	if _, ok := err.(*ValidationError); ok {
		t.Errorf("expected file error, got ValidationError: %v", err)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("HIVEMIND_CONFIG", "/nonexistent/config.toml")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() should fail for missing file")
	}
}

func TestLoad_MissingGatewayPort(t *testing.T) {
	cfg := `
[gateway]
admin_port   = 8401
metrics_port = 9090

[[backends]]
name  = "b"
url   = "http://localhost:11434"
model = "x"

[qdrant]
endpoint = "http://localhost:6333"
`
	path := writeTempConfig(t, cfg)
	t.Setenv("HIVEMIND_CONFIG", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected validation error for missing gateway.port")
	}
}

func TestLoad_NoBackends(t *testing.T) {
	cfg := `
[gateway]
port         = 8400
admin_port   = 8401
metrics_port = 9090

[qdrant]
endpoint = "http://localhost:6333"
`
	path := writeTempConfig(t, cfg)
	t.Setenv("HIVEMIND_CONFIG", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected validation error for missing backends")
	}
}

func TestLoad_PIIEnabledNoEndpoint(t *testing.T) {
	cfg := `
[gateway]
port         = 8400
admin_port   = 8401
metrics_port = 9090

[[backends]]
name  = "b"
url   = "http://localhost:11434"
model = "x"

[pii]
enabled    = true
timeout_ms = 500

[qdrant]
endpoint = "http://localhost:6333"
`
	path := writeTempConfig(t, cfg)
	t.Setenv("HIVEMIND_CONFIG", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected validation error for pii.endpoint when enabled")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	path := writeTempConfig(t, "not valid toml [[[")
	t.Setenv("HIVEMIND_CONFIG", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestValidationError_TypedError(t *testing.T) {
	ve := &ValidationError{Field: "test.field", Message: "required"}
	expected := `config: field "test.field": required`
	if ve.Error() != expected {
		t.Errorf("Error() = %q, want %q", ve.Error(), expected)
	}
}
