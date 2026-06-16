# HiveMindFusion

Multi-model AI gateway with fusion orchestration. Routes requests across local and cloud LLMs through a single OpenAI-compatible API, with multi-model consensus, PII filtering, RAG injection, and VRAM-aware model management.

## Architecture

```
Client (OpenAI SDK)
    │
    ▼
┌──────────────────────────────────────────┐
│  HiveMind Gateway (:8400)                │
│  ┌───────────┐  ┌────────────────┐       │
│  │ PII Shield│  │ RAG Middleware  │       │
│  └─────┬─────┘  └───────┬────────┘       │
│        ▼                ▼                │
│  ┌──────────────────────────────────┐    │
│  │   Router / Fallback Chains       │    │
│  └──┬───────┬───────┬───────┬───────┘    │
│     │       │       │       │            │
│     │  ┌────┴────────┴──────┐            │
│     │  │  Fusion Engine     │            │
│     │  │  (in-process)      │            │
│     │  │                    │            │
│     │  │  deliberate        │            │
│     │  │    ▼               │            │
│     │  │  fan-out ──────────┼──┐         │
│     │  │    ▼               │  │         │
│     │  │  judge             │  │         │
│     │  │    ▼               │  │         │
│     │  │  synthesize        │  │         │
│     │  └────────────────────┘  │         │
│     │                          │         │
└─────┼──────────────────────────┼─────────┘
      ▼                          ▼
   Ollama   Claude   Z.AI    (direct backend calls)
   :11434   Proxy    Cloud
```

**Gateway** (Go) — OpenAI-compatible reverse proxy with priority-based backend routing, health checks, consumer rate limiting, Prometheus metrics, and SIGHUP config reload.

**Fusion Engine** (Go, in-process) — Multi-model orchestrator that fans out prompts to panelist models, judges responses for consensus, and synthesizes a final answer. Runs inside the gateway process — no separate sidecar, no duplicate PII/RAG passes. Configurable tiers trade off quality vs cost.

## Features

- **OpenAI-compatible API** — drop-in replacement, works with any OpenAI SDK client
- **Multi-backend routing** — priority chains with automatic failover on health failure
- **Fusion consensus** — configurable panels of models deliberate, judge, and synthesize responses
- **PII filtering** — inline redaction via PII Shield before requests hit backends
- **RAG context injection** — per-consumer Qdrant vector search, injected as system context
- **VRAM management** — NVML-based calculator, hot-swap models without downtime
- **Rate limiting** — per-consumer burst and sustained request caps
- **Response compression** — optional sidecar for bandwidth savings on large outputs
- **Prometheus metrics** — request counts, latencies, errors per backend at `:8402/metrics`
- **SIGHUP reload** — update config without restarting the gateway

## Fusion Tiers

| Tier | Purpose | Panelists | Timeout |
|------|---------|-----------|---------|
| `frontier` | Maximum quality | 4 frontier models | 90s |
| `balanced` | Cost-quality balance | 4 mid-tier models | 60s |
| `budget` | Cheapest | 4 lightweight models | 45s |
| `test` | Mechanism validation | Working backends only | 60s |

Custom tiers can be defined in `fusion/panels.toml`.

## Quick Start

### Prerequisites

- Go 1.23+
- Python 3.10+ (for fusion sidecar)
- At least one LLM backend (Ollama, vLLM, cloud API, etc.)

### Build

```bash
go build -o hivemind-gw ./cmd/hivemind-gw
go build -o hivemind-ctl ./cmd/hivemind-ctl
```

### Configure

```bash
cp config.example.toml config.toml
# Edit config.toml — set your backend URLs and models
```

### Run

```bash
# Start the gateway (fusion runs in-process when enabled in config)
./hivemind-gw --config config.toml
```

### Use

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8400/v1",
    api_key=os.environ["HIVEMIND_KEY"],  # your consumer token
)

# Direct model request
response = client.chat.completions.create(
    model="qwopus3.6-27b-v2",
    messages=[{"role": "user", "content": "Hello"}]
)

# Fusion consensus request
response = client.chat.completions.create(
    model="hivemind/fusion-frontier",
    messages=[{"role": "user", "content": "Explain quantum entanglement"}]
)
```

## Configuration

Configuration is TOML-based. See `config.example.toml` for a minimal setup.

### Gateway

```toml
[gateway]
port         = 8400    # Main API port
admin_port   = 8401    # Health + admin endpoints
metrics_port = 8402    # Prometheus metrics
```

### Backends

```toml
[[backends]]
name            = "ollama-primary"
url             = "http://localhost:11434"
model           = "llama3.1:8b"
priority        = 1                    # Lower = preferred
health_endpoint = "/api/tags"
api_key_env     = "MY_API_KEY"         # Optional: env var name for API key
```

### PII Filtering

```toml
[pii]
endpoint          = "http://localhost:5100/scan"
enabled           = true
bypass_on_failure = false
timeout_ms        = 500
```

### RAG Context Injection

```toml
[qdrant]
endpoint   = "http://localhost:6333"
collection = "knowledge_graph"

[rag.consumers.my-app]
enabled    = true
collection = "knowledge_graph"
top_k      = 5
min_score  = 0.7
```

### Rate Limiting

```toml
[rate_limit.consumers.default]
requests_per_minute = 60
burst               = 10
```

## CLI Tool

`hivemind-ctl` manages models and VRAM:

```bash
# List available GGUF models
hivemind-ctl models list --dir /path/to/models

# Show model VRAM requirements
hivemind-ctl models info qwopus3.6-27b-v2

# Hot-swap models without downtime
hivemind-ctl models swap old-model new-model

# Show VRAM utilization
hivemind-ctl vram
```

## Endpoints

| Port | Path | Description |
|------|------|-------------|
| 8400 | `POST /v1/chat/completions` | Chat completions (streaming + non-streaming) |
| 8400 | `GET /v1/models` | List available models |
| 8401 | `GET /health` | Backend health status |
| 8402 | `GET /metrics` | Prometheus metrics |
| 8401 | `GET /panels` | Fusion tier configurations |

## Signals

| Signal | Action |
|--------|--------|
| `SIGHUP` | Reload configuration (backends, rate limits, RAG) without restart |
| `SIGTERM` | Graceful shutdown |

## Deployment

An OpenRC init script is included at `etc/openrc/hivemind` for systems using OpenRC. The gateway runs as a supervised daemon with automatic respawn.

```bash
# Install
sudo cp hivemind-gw /usr/local/bin/
sudo cp etc/openrc/hivemind /etc/init.d/
sudo rc-update add hivemind default
sudo rc-service hivemind start
```

## Project Structure

```
cmd/
  hivemind-gw/       Gateway server entrypoint
  hivemind-ctl/      CLI management tool
internal/
  config/            TOML config loader + validation
  fusion/            In-process fusion orchestrator (deliberate → fan-out → judge → synthesize)
  gateway/           Proxy, routing, health, metrics, consumers, fusion wiring
  models/            GGUF scanning, llama-server launcher, hot swap
  pii/               PII Shield client + redaction middleware
  rag/               Qdrant client + RAG injection middleware
  vram/              NVIDIA VRAM calculator (NVML)
config/
  batkave.toml       Production config example
config.example.toml  Minimal starter config
etc/openrc/          Init scripts
```

## License

AGPL-3.0 with commercial restriction. See [LICENSE](LICENSE).

Free to use, modify, and redistribute for non-commercial purposes. Commercial use requires written permission from the copyright holder.
