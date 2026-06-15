"""HiveMind Fusion — OpenAI-compatible model blending service on :8500."""

import json
import logging
import sys
from typing import Any

import uvicorn
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse, StreamingResponse

from config import load_panels
from orchestrator import run_fusion, run_fusion_stream

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [fusion] %(levelname)s %(name)s %(message)s",
    stream=sys.stdout,
)
log = logging.getLogger("server")

app = FastAPI(title="HiveMind Fusion", version="0.1.0")


@app.get("/health")
async def health():
    panels = load_panels().get("panels", {})
    return {
        "status": "ok",
        "service": "hivemind-fusion",
        "tiers": list(panels.keys()),
    }


@app.get("/panels")
async def list_panels():
    panels = load_panels().get("panels", {})
    return {
        "panels": {
            name: {
                "panelists": cfg.get("panelists", []),
                "judge": cfg.get("judge", ""),
                "synth": cfg.get("synth", ""),
            }
            for name, cfg in panels.items()
        }
    }


@app.get("/v1/models")
async def list_models():
    panels = load_panels().get("panels", {})
    return {
        "object": "list",
        "data": [
            {"id": f"hivemind/fusion-{name}", "object": "model", "owned_by": "hivemind"}
            for name in panels
        ],
    }


@app.post("/v1/chat/completions")
async def chat_completions(request: Request):
    body: dict[str, Any] = await request.json()
    model = body.get("model", "")
    messages = body.get("messages", [])
    stream = body.get("stream", False)

    if not messages:
        return JSONResponse(
            status_code=400,
            content={"error": {"message": "messages field required", "type": "invalid_request"}},
        )

    # Accept both "hivemind/fusion-X" and bare "fusion-X" or "X"
    tier = model
    panels = load_panels().get("panels", {})
    for prefix in ("hivemind/fusion-", "fusion-"):
        if tier.startswith(prefix):
            tier = "hivemind/fusion-" + tier[len(prefix):]
            break
    else:
        # Bare tier name (e.g., "balanced") → normalize
        if model in panels:
            tier = f"hivemind/fusion-{model}"

    if tier.replace("hivemind/fusion-", "") not in panels:
        return JSONResponse(
            status_code=404,
            content={
                "error": {
                    "message": f"unknown fusion tier: {model}",
                    "available": [f"hivemind/fusion-{t}" for t in panels],
                    "type": "invalid_request",
                }
            },
        )

    log.info("request: model=%s stream=%s msgs=%d", tier, stream, len(messages))

    if stream:
        return StreamingResponse(
            run_fusion_stream(tier, messages),
            media_type="text/event-stream",
            headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
        )

    try:
        result = await run_fusion(tier, messages)
        if "error" in result:
            return JSONResponse(status_code=502, content=result)
        # Keep _meta in response — harmless extra field, valuable for observability
        return JSONResponse(content=result)
    except Exception as e:
        log.exception("fusion error")
        return JSONResponse(
            status_code=500,
            content={"error": {"message": str(e), "type": "internal_error"}},
        )


if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8500, log_level="info")
