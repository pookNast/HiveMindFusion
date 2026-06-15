"""Claude CLI Proxy — exposes `claude -p` headless as an OpenAI-compatible API on :8095."""

import json
import logging
import sys
from typing import Any

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse, StreamingResponse
import uvicorn

from cli_wrapper import call_claude, call_claude_streaming, CLIError

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [claude-cli-proxy] %(levelname)s %(message)s",
    stream=sys.stdout,
)
log = logging.getLogger(__name__)

app = FastAPI(title="Claude CLI Proxy", version="0.1.0")


@app.get("/health")
async def health():
    return {"status": "ok", "service": "claude-cli-proxy"}


@app.get("/v1/models")
async def list_models():
    from cli_wrapper import MODEL_MAP
    return {
        "object": "list",
        "data": [
            {"id": slug, "object": "model", "owned_by": "anthropic"}
            for slug in MODEL_MAP
        ],
    }


@app.post("/v1/chat/completions")
async def chat_completions(request: Request):
    body: dict[str, Any] = await request.json()
    model = body.get("model", "claude-sonnet-4-6")
    messages = body.get("messages", [])
    stream = body.get("stream", False)
    max_tokens = body.get("max_tokens")
    temperature = body.get("temperature")

    if not messages:
        return JSONResponse(
            status_code=400,
            content={"error": {"message": "messages field required", "type": "invalid_request"}},
        )

    if stream:
        return StreamingResponse(
            call_claude_streaming(
                model, messages, max_tokens=max_tokens
            ),
            media_type="text/event-stream",
        )

    try:
        result = call_claude(
            model, messages,
            max_tokens=max_tokens,
            temperature=temperature,
        )
        log.info(
            "model=%s tokens=%d elapsed_ms=%d cost=$%.4f",
            model,
            result["usage"]["total_tokens"],
            result["_meta"]["elapsed_ms"],
            result["_meta"]["cost_usd"],
        )
        # Strip _meta from response (internal only)
        result.pop("_meta", None)
        return JSONResponse(content=result)

    except CLIError as e:
        log.error("CLI error: %s", e)
        return JSONResponse(
            status_code=502,
            content={"error": {"message": str(e), "type": "upstream_error"}},
        )


if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8095, log_level="info")
