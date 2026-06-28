# HiveMind Model Overlays

Per-agent behavioral patches for the homelab's HiveMind gateway
(`http://127.0.0.1:8400`). This is gstack's overlay pattern adapted to
HiveMind's consumer-tag identity model.

## What an overlay is

Each `.md` file is a behavioral system prompt for one agent role: role
definition, core directives, decision principles, communication style,
constraints, and integration points. ~30-40 lines, behavioral not verbose.

## How HiveMind consumes them

HiveMind has **no system-prompt injection endpoint**. Consumer identity travels
via the `X-HiveMind-Consumer` HTTP header
([`internal/gateway/consumer.go:Identify`](../internal/gateway/consumer.go)).
So an overlay is a **client-side contract**: the agent must (a) set its consumer
header, and (b) prepend `dist/<tag>.txt` as the `system` message in its chat
request. The gateway then applies the right rate limits, RAG context, and PII
filtering for that consumer.

## Files

| Overlay | Consumer tag |
|---|---|
| `ceo.md` | `ceo-agent` |
| `cfo.md` | `cfo-agent` |
| `cto.md` | `cto-agent` |
| `rosie.md` | `rosie` |
| `super-worker.md` | `super-worker` |
| `loop-maxxer.md` | `loop-maxxer` |
| `siteops.md` | `siteops` |

`apply-overlays.sh` — materializes each `.md` into `dist/<tag>.txt` (heading
markup stripped) and emits `dist/MANIFEST.tsv`. Validates every tag against the
live gateway config (`config/batkave.toml`) and flags any consumer not yet
registered in `[rate_limit.consumers.*]` / `[rag.consumers.*]`. Exits non-zero
if any tag is unknown — usable as a preflight gate.

`dist/` — generated; do not edit. Source of truth is the `.md` files.

## Usage

```bash
# build / re-validate (run on boot, after overlay edits, or post gateway reload)
~/ralph/hivemind/overlays/apply-overlays.sh

# agent-side: read the materialized system prompt + set the consumer header
TAG=ceo-agent
SYS="$(cat ~/ralph/hivemind/overlays/dist/$TAG.txt)"
curl http://127.0.0.1:8400/v1/chat/completions \
  -H "X-HiveMind-Consumer: $TAG" \
  -H 'Content-Type: application/json' \
  -d "{\"model\":\"glm-5.1\",\"messages\":[{\"role\":\"system\",\"content\":\"$SYS\"},{\"role\":\"user\",\"content\":\"...\"}]}"
```

## Registering a new consumer tag

A tag appearing in WARN means it isn't in the gateway config yet. Add it:

```toml
[rate_limit.consumers.<tag>]
requests_per_minute = 60
burst               = 10

[rag.consumers.<tag>]        # optional: Qdrant context injection
enabled    = true
collection = "knowledge_graph"
top_k      = 3
min_score  = 0.75
```

Then `hivemind-ctl reload` and re-run `apply-overlays.sh`.

## Editing overlays

Change the `.md`, re-run `apply-overlays.sh`. Keep overlays behavioral and
~30-50 lines (ponytail `full`). Constraints and security boundaries are exempt
from minimization — never cut them.
